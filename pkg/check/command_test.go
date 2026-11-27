package check_test

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samcontesse/gitlab-merge-request-resource/pkg"
	"github.com/samcontesse/gitlab-merge-request-resource/pkg/check"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"
)

var _ = Describe("Check", func() {

	var (
		t                   time.Time
		mux                 *http.ServeMux
		command             *check.Command
		root                *url.URL
		detailedMergeStatus string
		draft               bool
	)

	BeforeEach(func() {
		t, _ = time.Parse(time.RFC3339, "2022-01-01T08:00:00Z")
		detailedMergeStatus = "mergeable"
		draft = false
		mux = http.NewServeMux()
		server := httptest.NewServer(mux)
		root, _ = url.Parse(server.URL)
		context, _ := url.Parse("/api/v4")
		base := root.ResolveReference(context)
		client, _ := gitlab.NewClient("$", gitlab.WithBaseURL(base.String()))

		_ = os.Setenv("ATC_EXTERNAL_URL", "https://concourse-ci.company.ltd")
		_ = os.Setenv("BUILD_TEAM_NAME", "winner")
		_ = os.Setenv("BUILD_PIPELINE_NAME", "baltic")
		_ = os.Setenv("BUILD_JOB_NAME", "release")
		_ = os.Setenv("BUILD_NAME", "1")

		command = check.NewCommand(client)
	})

	Describe("Run", func() {

		Context("When it has a minimal valid configuration", func() {

			BeforeEach(func() {
				mux.HandleFunc("/api/v4/projects/namespace%2Fproject/merge_requests", func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					Expect(r.Method).To(Equal(http.MethodGet))
					Expect(r.URL.EscapedPath()).To(Equal("/api/v4/projects/namespace%2Fproject/merge_requests"))
					Expect(r.Header.Get("Private-Token")).To(Equal("$"))
					Expect(r.URL.Query().Get("state")).To(Equal("opened"))
					mr := gitlab.BasicMergeRequest{IID: 88, ID: 99, SHA: "abc", ProjectID: 42, DetailedMergeStatus: detailedMergeStatus, Draft: draft}
					output, _ := json.Marshal([]gitlab.BasicMergeRequest{mr})
					w.Header().Set("content-type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, err := w.Write(output)
					Expect(err).NotTo(HaveOccurred())
				})

				mux.HandleFunc("/api/v4/projects/42/repository/commits/abc", func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					commit := gitlab.Commit{CommittedDate: &t}
					output, _ := json.Marshal(commit)
					w.Header().Set("content-type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, err := w.Write(output)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			It("Should return a single version", func() {

				project, _ := url.Parse("namespace/project.git")
				uri := root.ResolveReference(project)

				request := check.Request{
					Source: pkg.Source{
						URI:          uri.String(),
						PrivateToken: "$",
					},
				}

				response, err := command.Run(request)
				Expect(err).Should(BeNil())
				Expect(len(response)).To(Equal(1))
				Expect(response[0].ID).To(Equal(int64(88)))
				Expect(response[0].UpdatedAt).To(Equal(&t))
			})

			It("filters merge requests according to their merge status", func() {
				project, _ := url.Parse("namespace/project.git")
				uri := root.ResolveReference(project)
				request := check.Request{
					Source: pkg.Source{
						URI:              uri.String(),
						PrivateToken:     "$",
						SkipNotMergeable: true,
					},
				}

				response, err := command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveLen(1))

				detailedMergeStatus = "conflict"
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(BeEmpty())

				detailedMergeStatus = "checking"
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(BeEmpty())

				request.Source.SkipNotMergeable = false
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveLen(1))
			})

			It("skips drafts only when requested", func() {
				draft = true
				request := check.Request{
					Source: pkg.Source{
						URI:                root.String() + "/namespace/project.git",
						SkipWorkInProgress: true,
					},
				}

				response, err := command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(BeEmpty())

				request.Source.SkipWorkInProgress = false
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveLen(1))

				draft = false
				request.Source.SkipWorkInProgress = true
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveLen(1))
			})

			It("filters paths using merge request diff versions", func() {
				mux.HandleFunc("/api/v4/projects/42/merge_requests/88/versions", func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					Expect(json.NewEncoder(w).Encode([]gitlab.MergeRequestDiffVersion{{ID: 7}})).To(Succeed())
				})
				mux.HandleFunc("/api/v4/projects/42/merge_requests/88/versions/7", func(w http.ResponseWriter, r *http.Request) {
					defer GinkgoRecover()
					version := gitlab.MergeRequestDiffVersion{
						ID:    7,
						Diffs: []*gitlab.Diff{{OldPath: "src/main.go", NewPath: "src/main.go"}},
					}
					Expect(json.NewEncoder(w).Encode(version)).To(Succeed())
				})
				request := check.Request{
					Source: pkg.Source{
						URI:   root.String() + "/namespace/project.git",
						Paths: []string{"src/*"},
					},
				}

				response, err := command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveLen(1))

				request.Source.Paths = []string{"docs/*"}
				response, err = command.Run(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(BeEmpty())
			})

		})

		Context("When it contains an invalid project uri", func() {

			BeforeEach(func() {
				mux.HandleFunc("/api/v4/projects/invalid%2Fproject/merge_requests", http.NotFound)
			})

			It("Should error when project uri is invalid", func() {

				project, _ := url.Parse("invalid/project.git")
				uri := root.ResolveReference(project)

				request := check.Request{
					Source: pkg.Source{
						URI:          uri.String(),
						PrivateToken: "$",
					},
				}

				response, err := command.Run(request)
				Expect(response).To(Equal(check.Response{}))
				Expect(err).NotTo(BeNil())
			})

		})

	})

})
