package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gnym/config"
	"gnym/input"
	"gnym/review"
	"gnym/reviewer"
	"gnym/sink"
	"io"
	"net/url"
	"os"
	"strings"
)

const version = "0.1.0"

type reviewerOptions struct {
	Model string `json:"model,omitempty"`
}

type fileSinkOptions struct {
	Path string `json:"path"`
}

type diffSourceFactory func(source *url.URL) (review.DiffSource, error)
type reviewerFactory func(configured config.Reviewer) (review.Reviewer, reviewer.Config, error)
type sinkFactory func(options json.RawMessage) (review.CommentSink, error)

var diffSourceFactories = map[string]diffSourceFactory{
	"file": newFileDiffSource,
}

var reviewerFactories = map[string]reviewerFactory{
	"stub": newStubReviewer,
}

var sinkFactories = map[string]sinkFactory{
	"file": newFileSink,
}

type reviewerRouter map[string]review.Reviewer

func (r reviewerRouter) Review(request reviewer.Request) (reviewer.Payload, error) {
	configuredReviewer, ok := r[request.Config.Name]
	if !ok {
		return reviewer.Payload{}, fmt.Errorf("reviewer %q is not registered", request.Config.Name)
	}
	return configuredReviewer.Review(request)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "gnym:", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		return errors.New("expected a command: review or version")
	}

	switch arguments[0] {
	case "review":
		return runReview(arguments[1:], stderr)
	case "version":
		if len(arguments) != 1 {
			return errors.New("version does not accept arguments")
		}
		fmt.Fprintf(stdout, "gnym v%s\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func runReview(arguments []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("review", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to the JSON configuration file")
	diffSource := flags.String("diff-source", "", "diff source URI, for example file://changes.diff")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("review does not accept positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *configPath == "" {
		return errors.New("review requires --config")
	}
	if *diffSource == "" {
		return errors.New("review requires --diff-source")
	}

	configured, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	source, err := buildDiffSource(*diffSource)
	if err != nil {
		return err
	}
	router, reviewerConfigs, err := buildReviewers(configured.Reviewers)
	if err != nil {
		return err
	}
	commentSink, err := buildSink(configured.Sink)
	if err != nil {
		return err
	}

	coordinator := review.NewCoordinator(source, router, reviewerConfigs, commentSink)
	return coordinator.Run()
}

func buildDiffSource(reference string) (review.DiffSource, error) {
	parsed, err := url.Parse(reference)
	if err != nil {
		return nil, fmt.Errorf("parse diff source: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, errors.New("diff source must include a scheme, for example file://changes.diff")
	}
	factory, ok := diffSourceFactories[parsed.Scheme]
	if !ok {
		return nil, fmt.Errorf("diff source type %q is not registered", parsed.Scheme)
	}
	return factory(parsed)
}

func newFileDiffSource(source *url.URL) (review.DiffSource, error) {
	path, err := fileSourcePath(source)
	if err != nil {
		return nil, err
	}
	return &input.FileDiffSource{Path: path}, nil
}

func fileSourcePath(source *url.URL) (string, error) {
	var path string
	switch {
	case source.Opaque != "":
		path = source.Opaque
	case source.Host != "" && source.Path == "":
		path = source.Host
	case source.Host == "" && source.Path != "":
		path = source.Path
	case source.Opaque == "" && source.Host == "" && source.Path == "":
		return "", errors.New("file diff source requires a path")
	default:
		return "", fmt.Errorf("invalid file diff source %q", source.String())
	}
	return path, nil
}

func buildReviewers(configured []config.Reviewer) (reviewerRouter, []reviewer.Config, error) {
	router := make(reviewerRouter)
	reviewerConfigs := make([]reviewer.Config, 0, len(configured))

	for _, configuredReviewer := range configured {
		factory, ok := reviewerFactories[configuredReviewer.Type]
		if !ok {
			return nil, nil, fmt.Errorf("reviewer type %q is not registered", configuredReviewer.Type)
		}
		builtReviewer, reviewerConfig, err := factory(configuredReviewer)
		if err != nil {
			return nil, nil, fmt.Errorf("reviewer %q: %w", configuredReviewer.Name, err)
		}
		router[configuredReviewer.Name] = builtReviewer
		reviewerConfigs = append(reviewerConfigs, reviewerConfig)
	}

	return router, reviewerConfigs, nil
}

func newStubReviewer(configured config.Reviewer) (review.Reviewer, reviewer.Config, error) {
	options := reviewerOptions{}
	if err := decodeOptions(configured.Options, &options); err != nil {
		return nil, reviewer.Config{}, fmt.Errorf("options: %w", err)
	}
	return &reviewer.Stub{}, reviewer.Config{
		Name:     configured.Name,
		Provider: configured.Type,
		Model:    options.Model,
		Prompt:   configured.Prompt,
	}, nil
}

func buildSink(configured config.Sink) (review.CommentSink, error) {
	factory, ok := sinkFactories[configured.Type]
	if !ok {
		return nil, fmt.Errorf("sink type %q is not registered", configured.Type)
	}
	return factory(configured.Options)
}

func newFileSink(rawOptions json.RawMessage) (review.CommentSink, error) {
	options := fileSinkOptions{}
	if err := decodeOptions(rawOptions, &options); err != nil {
		return nil, fmt.Errorf("file sink options: %w", err)
	}
	if strings.TrimSpace(options.Path) == "" {
		return nil, errors.New("file sink requires options.path")
	}
	return &sink.File{Path: options.Path}, nil
}

func decodeOptions(raw json.RawMessage, destination any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return nil
}
