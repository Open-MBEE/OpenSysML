package hygiene

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestDockerfileBuildsWithThePinnedToolchain holds the image's build stage to
// the toolchain go.mod pins, so the image's binaries are compiled by the same
// Go as the release archives and a toolchain bump that forgets the Dockerfile
// fails here rather than in the release.
func TestDockerfileBuildsWithThePinnedToolchain(t *testing.T) {
	raw, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	pinned := regexp.MustCompile(`(?m)^toolchain\s+go(\S+)\s*$`).FindStringSubmatch(string(raw))
	if pinned == nil {
		t.Fatal("go.mod pins no toolchain; the Dockerfile's build stage needs one to follow")
	}
	want := "golang:" + pinned[1] + "-alpine"

	dockerfile, err := os.Open("../../Dockerfile")
	if err != nil {
		t.Fatalf("open Dockerfile: %v", err)
	}
	defer dockerfile.Close()
	from := regexp.MustCompile(`^FROM\s+(?:--platform=\S+\s+)?(\S+)\s+AS build$`)
	scanner := bufio.NewScanner(dockerfile)
	for scanner.Scan() {
		if m := from.FindStringSubmatch(strings.TrimSpace(scanner.Text())); m != nil {
			if m[1] != want {
				t.Errorf("Dockerfile build stage runs %s; go.mod pins toolchain go%s, so it must run %s", m[1], pinned[1], want)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	t.Fatal("Dockerfile has no `FROM … AS build` stage")
}

// TestContainerImageIsVerifiedBeforeItIsPushed holds publish-docker to the
// order the image depends on: the tag is resolved, the credentials are
// required, a published tag is refused, the linux/amd64 image is built, run
// and asked for the release version and its health, and only then are both
// platforms built and pushed under every tag.
func TestContainerImageIsVerifiedBeforeItIsPushed(t *testing.T) {
	config := loadCircleConfig(t)
	job, ok := config.Jobs["publish-docker"]
	if !ok {
		t.Fatal("no publish-docker job")
	}
	if !config.jobRunsOnImage(t, "publish-docker", "cimg/base:current") {
		t.Error("publish-docker does not run on cimg/base:current, the image that carries the Docker CLI with buildx")
	}
	if !stepHasBareStep(job.Steps, "setup_remote_docker") {
		t.Error("publish-docker does not attach a remote Docker host")
	}
	runs := runSteps(t, job.Steps)
	login := stepIndex(runs, "docker login", "--password-stdin")
	refuse := stepIndex(runs, "/v2/${image}/manifests/${IMAGE_VERSION}", "404)", "200)", "*)")
	check := stepIndex(runs, "--platform linux/amd64 --load", "-version", "/health", "GetServerInfo")
	push := stepIndex(runs, "--platform linux/amd64,linux/arm64 --push")
	rehearse := stepIndex(runs, "--platform linux/amd64,linux/arm64", "push=false")
	if login < 0 {
		t.Fatal("publish-docker does not log in to Docker Hub with --password-stdin")
	}
	if refuse < 0 {
		t.Fatal("publish-docker does not ask the registry for the version tag and fail closed on anything but a definite 404")
	}
	if check < 0 {
		t.Fatal("publish-docker does not run the linux/amd64 image and check its version, /health and a Connect call")
	}
	if push < 0 {
		t.Fatal("publish-docker does not push a linux/amd64,linux/arm64 image")
	}
	if rehearse < 0 {
		t.Fatal("publish-docker does not build both platforms, unpushed, in a rehearsal")
	}
	if !(login < refuse && refuse < check && check < push) {
		t.Errorf("publish-docker runs login, refusal, check, push at steps %d, %d, %d, %d; the push must come last", login, refuse, check, push)
	}
	for _, fragment := range []string{"--build-arg VERSION=\"${CIRCLE_TAG}\"", "--build-arg COMMIT=\"${CIRCLE_SHA1}\"", "--build-arg BUILD_TIME=\"${BUILD_TIME}\""} {
		if !strings.Contains(runs[push].Command, fragment) {
			t.Errorf("the pushed image is not built with %s, so its binaries would not carry the release's metadata", fragment)
		}
	}
	if !containsAll(runs[push].Command, "${tag_flags}") {
		t.Error("the push does not tag the image with every resolved tag")
	}
	if stepIndex(runs, "export IMAGE_VERSION=${version}") < 0 {
		t.Error("publish-docker does not resolve the image version from the tag")
	}
	for _, fragment := range []string{"openmbee/opensysml:${IMAGE_VERSION}", "openmbee/opensysml:latest", "*-*)", "git fetch --tags", "sort -V"} {
		if !strings.Contains(runs[push].Command, fragment) {
			t.Errorf("the push step does not decide the tags itself with %q; a decision taken earlier could roll latest back", fragment)
		}
	}

	release := config.workflow(t, "release")
	requireAll(t, "Publish openmbee/opensysml to Docker Hub", requiresOf(t, release, "Publish openmbee/opensysml to Docker Hub"), "Publish GitHub release")
}
