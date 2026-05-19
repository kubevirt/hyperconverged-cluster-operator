package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type jobConfig struct {
	name    string
	baseURL string
}

var jobs = []jobConfig{
	{
		name:    "Nightly build on kubevirtci",
		baseURL: "https://storage.googleapis.com/kubevirt-prow/logs/periodic-hco-push-nightly-build-main",
	},
	{
		name:    "Nightly deploy on OCP",
		baseURL: "https://storage.googleapis.com/test-platform-results/logs/periodic-ci-kubevirt-hyperconverged-cluster-operator-main-hco-e2e-deploy-nightly-main-aws",
	},
}

var (
	successEmoji = slack.NewRichTextSectionEmojiElement("solid-success", 3, nil)
	failedEmoji  = slack.NewRichTextSectionEmojiElement("failed", 3, nil)
)

type finished struct {
	Timestamp int64  `json:"timestamp"`
	Passed    bool   `json:"passed"`
	Result    string `json:"result"`
	Revision  string `json:"revision"`
}

var (
	token     string
	channelId string
	groupId   string
)

func init() {
	var ok bool
	token, ok = os.LookupEnv("HCO_REPORTER_SLACK_TOKEN")
	if !ok {
		fmt.Fprintln(os.Stderr, "HCO_REPORTER_SLACK_TOKEN environment variable not set")
		os.Exit(1)
	}

	channelId, ok = os.LookupEnv("HCO_CHANNEL_ID")
	if !ok {
		fmt.Fprintln(os.Stderr, "HCO_CHANNEL_ID environment variable not set")
		os.Exit(1)
	}

	groupId, ok = os.LookupEnv("HCO_GROUP_ID")
	if !ok {
		fmt.Fprintln(os.Stderr, "HCO_GROUP_ID environment variable not set")
		os.Exit(1)
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	blocks, err := generateMessage(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = sendMessageToSlackChannel(blocks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to send the message to the channel; ", err.Error())
		if serr, ok := err.(slack.SlackErrorResponse); ok {
			for _, msg := range serr.ResponseMetadata.Messages {
				fmt.Fprintln(os.Stderr, msg)
			}
		}
		os.Exit(1)
	}

	fmt.Println("Successfully sent message to the channel")
}

func generateMessage(ctx context.Context) ([]slack.Block, error) {
	client := http.DefaultClient
	client.Timeout = time.Second * 3

	var allBlocks []slack.Block
	needMention := false
	successCount := 0

	for i, job := range jobs {
		if i > 0 {
			allBlocks = append(allBlocks, slack.NewDividerBlock())
		}

		blocks, shouldMention, err := generateJobMessage(ctx, client, job)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get status for %q: %s\n", job.name, err.Error())
			blocks = generateErrorMessage(job.name, err)
			shouldMention = true
		} else {
			successCount++
		}

		allBlocks = append(allBlocks, blocks...)
		if shouldMention {
			needMention = true
		}
	}

	if successCount == 0 && len(allBlocks) == 0 {
		return nil, fmt.Errorf("failed to fetch status for all jobs")
	}

	if needMention {
		allBlocks = append(allBlocks, generateMentionBlock())
	}

	return allBlocks, nil
}

func generateJobMessage(ctx context.Context, client *http.Client, job jobConfig) ([]slack.Block, bool, error) {
	latestBuild, err := getLatestBuild(ctx, client, job.baseURL)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get latest job ID; %s", err.Error())
	}

	buildStatus, err := getBuildStatus(ctx, client, latestBuild, job.baseURL)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch the build status; %s", err.Error())
	}

	buildTime := time.Unix(buildStatus.Timestamp, 0).UTC()
	if time.Since(buildTime).Hours() > 24 {
		return generateNoBuildMessage(job.name, buildTime), true, nil
	}

	jobURL, err := getJob(ctx, client, latestBuild, job.baseURL)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch the job info; %s", err.Error())
	}

	blocks, shouldMention := generateStatusMessage(job.name, buildStatus, buildTime, jobURL)
	return blocks, shouldMention, nil
}

func sendMessageToSlackChannel(blocks []slack.Block) error {
	s := slack.New(token)
	_, _, err := s.PostMessage(channelId, slack.MsgOptionBlocks(blocks...))
	return err
}

func generateMentionBlock() slack.Block {
	return slack.NewRichTextBlock("mention", slack.NewRichTextSection(
		slack.NewRichTextSectionUserGroupElement(groupId),
	))
}

func generateErrorMessage(jobName string, fetchErr error) []slack.Block {
	return []slack.Block{
		slack.NewRichTextBlock("", slack.NewRichTextSection(
			failedEmoji,
			slack.NewRichTextSectionTextElement(
				fmt.Sprintf(" %s: failed to fetch status: %s", jobName, fetchErr.Error()), nil,
			),
		)),
	}
}

func generateNoBuildMessage(jobName string, buildTime time.Time) []slack.Block {
	return []slack.Block{
		slack.NewRichTextBlock("", slack.NewRichTextSection(
			failedEmoji,
			slack.NewRichTextSectionTextElement(
				fmt.Sprintf(" %s wasn't run today", jobName), nil,
			),
		)),
		slack.NewRichTextBlock("", slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement("Last build was at ", nil),
			slack.NewRichTextSectionDateElement(buildTime.UTC().Unix(), "{date_long_full} at {time}, {ago}", nil, nil),
		)),
	}
}

func generateStatusMessage(jobName string, buildStatus *finished, buildTime time.Time, jobURL string) ([]slack.Block, bool) {
	var (
		status string
		emoji  *slack.RichTextSectionEmojiElement
	)
	if buildStatus.Passed {
		status = "passed"
		emoji = successEmoji
	} else {
		status = "failed"
		emoji = failedEmoji
	}

	blocks := []slack.Block{
		slack.NewRichTextBlock("", slack.NewRichTextSection(
			emoji,
			slack.NewRichTextSectionTextElement(
				fmt.Sprintf(" %s ", jobName), nil,
			),
			slack.NewRichTextSectionLinkElement(jobURL, status, &slack.RichTextSectionTextStyle{Bold: true}),
			slack.NewRichTextSectionTextElement(", at ", nil),
			slack.NewRichTextSectionDateElement(buildTime.UTC().Unix(), "{date_long_full} at {time}", nil, nil),
		)),
	}

	return blocks, !buildStatus.Passed
}

func getLatestBuild(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/latest-build.txt", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if err = checkHTTPStatusCode(resp); err != nil {
		return "", fmt.Errorf("latest build request failed; %w", err)
	}

	latestBuildBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	latestBuild := strings.TrimSpace(string(latestBuildBytes))
	if latestBuild == "" {
		return "", fmt.Errorf("latest build is empty")
	}

	return string(latestBuildBytes), nil
}

func getBuildStatus(ctx context.Context, client *http.Client, latestBuild string, baseURL string) (*finished, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/finished.json", baseURL, latestBuild), nil)
	if err != nil {
		return nil, err
	}

	finishedResp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer finishedResp.Body.Close()
	if err = checkHTTPStatusCode(finishedResp); err != nil {
		return nil, fmt.Errorf("failed to get finished response; %w", err)
	}

	f := &finished{}
	dec := json.NewDecoder(finishedResp.Body)
	if err = dec.Decode(&f); err != nil {
		return nil, err
	}
	return f, nil
}

func getJob(ctx context.Context, client *http.Client, latestBuild string, baseURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s/prowjob.json", baseURL, latestBuild), nil)
	if err != nil {
		return "", err
	}

	jobResp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return "", err
	}

	defer jobResp.Body.Close()
	if err = checkHTTPStatusCode(jobResp); err != nil {
		return "", fmt.Errorf("failed to get job details; %w", err)
	}

	job := struct {
		Status struct {
			URL string `json:"url,omitempty"`
		} `json:"status"`
	}{}
	dec := json.NewDecoder(jobResp.Body)
	err = dec.Decode(&job)
	if err != nil {
		return "", err
	}
	return job.Status.URL, nil
}

func checkHTTPStatusCode(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP status is not OK; %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}
