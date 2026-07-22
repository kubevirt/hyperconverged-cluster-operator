package tests_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	v1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/downloadhost"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	tests "github.com/kubevirt/hyperconverged-cluster-operator/tests/func-tests"
)

var _ = Describe("[rfe_id:5100][crit:medium][vendor:cnv-qe@redhat.com][level:system]HyperConverged Cluster Operator should create ConsoleCliDownload objects", Label(tests.OpenshiftLabel, "ConsoleCliDownload"), func() {
	flag.Parse()

	var (
		cli client.Client
	)

	BeforeEach(func(ctx context.Context) {
		tests.BeforeEach(ctx)
		cfg, err := config.GetConfig()
		Expect(err).ToNot(HaveOccurred())

		s := scheme.Scheme
		Expect(consolev1.AddToScheme(s)).To(Succeed())
		cli, err = client.New(cfg, client.Options{Scheme: s})
		Expect(err).ToNot(HaveOccurred())

		tests.FailIfNotOpenShift(ctx, cli, "ConsoleCliDownload")
	})

	It("[test_id:6956]should create ConsoleCliDownload objects with expected spec", Label("test_id:6956"), func(ctx context.Context) {
		By("Checking existence of ConsoleCliDownload")

		ccd := &consolev1.ConsoleCLIDownload{
			ObjectMeta: metav1.ObjectMeta{
				Name: "virtctl-clidownloads-kubevirt-hyperconverged",
			},
		}

		Expect(cli.Get(ctx, client.ObjectKeyFromObject(ccd), ccd)).To(Succeed())

		Expect(ccd.Spec.Links).To(HaveLen(7))

		httpClient := getHTTPClient()

		for _, link := range ccd.Spec.Links {
			// virtctl for Windows for ARM 64 is still not shipped, avoid checking it
			// TODO: remove this once ready
			if strings.Contains(link.Href, "windows") && strings.Contains(link.Href, "arm64") {
				continue
			}
			By("Checking links. Link:" + link.Href)
			resp, err := httpClient.Head(link.Href)

			Expect(err).NotTo(HaveOccurred(), "HEAD failed for %s", link.Href)
			Expect(resp.StatusCode).To(Equal(http.StatusOK), "non OK response for %s", link.Href)

			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			ExpectWithOffset(1, resp).To(HaveHTTPStatus(http.StatusOK))
		}

		link, err := url.Parse(ccd.Spec.Links[0].Href)
		Expect(err).ToNot(HaveOccurred())

		checkVirtioWinDownload(ctx, cli, httpClient, link.Host)
	})

	Context("URL Download customization", func() {
		var existingRoutes string
		BeforeEach(func(ctx context.Context) {
			GinkgoWriter.Println("Reading the Ingress before")
			ingress := &configv1.Ingress{}
			Expect(cli.Get(ctx, client.ObjectKey{Name: "cluster"}, ingress)).To(Succeed())

			routeBytes, err := json.Marshal(ingress.Spec.ComponentRoutes)
			Expect(err).ToNot(HaveOccurred())
			existingRoutes = string(routeBytes)

			if len(ingress.Spec.ComponentRoutes) > 0 {
				GinkgoWriter.Println("removing the custom routes before the test")
				cleanupPatch := []byte(`[{"op": "remove", "path": "/spec/componentRoutes"}]`)
				Expect(cli.Patch(ctx, ingress, client.RawPatch(types.JSONPatchType, cleanupPatch))).To(Succeed())
			}
		})

		AfterEach(func(ctx context.Context) {
			ingress := &configv1.Ingress{}
			Expect(cli.Get(ctx, client.ObjectKey{Name: "cluster"}, ingress)).To(Succeed())

			if len(ingress.Spec.ComponentRoutes) > 0 {
				GinkgoWriter.Println("restoring the custom routes after the test")
				cleanupPatch := []byte(fmt.Sprintf(`[{"op": "replace", "path": "/spec/componentRoutes", "value": %v}]`, existingRoutes))
				Expect(cli.Patch(ctx, ingress, client.RawPatch(types.JSONPatchType, cleanupPatch))).To(Succeed())
			}
		})

		It("should allow download URL customization", Label("custom_dl_link"), func(ctx context.Context) {
			By("make sure the ingress contains the virt-downloads route component")
			ingress := &configv1.Ingress{}
			Expect(cli.Get(ctx, client.ObjectKey{Name: "cluster"}, ingress)).To(Succeed())

			Expect(slices.ContainsFunc(ingress.Status.ComponentRoutes, func(route configv1.ComponentRouteStatus) bool {
				return route.Name == "virt-downloads"
			})).To(BeTrueBecause("can't find the virt-downloads route in status of the the cluster Ingress"))

			By("customize the virt-downloads route, to set another host")
			baseDomain := ingress.Spec.Domain
			newCLIDLHost := "virt-dl." + baseDomain
			patch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/spec/componentRoutes", "value": [{"name": "virt-downloads", "hostname": %q, "namespace": %q}]}]`, newCLIDLHost, tests.InstallNamespace))
			Expect(cli.Patch(ctx, ingress, client.RawPatch(types.JSONPatchType, patch))).To(Succeed())

			httpClient := getHTTPClient()

			ccdKey := client.ObjectKey{Name: "virtctl-clidownloads-kubevirt-hyperconverged"}

			By("checking ConsoleCLIDownload links")
			Eventually(func(g Gomega, ctx context.Context) {
				ccd := &consolev1.ConsoleCLIDownload{}

				g.Expect(cli.Get(ctx, ccdKey, ccd)).To(Succeed())
				g.Expect(ccd.Spec.Links).To(HaveLen(7))
				for _, link := range ccd.Spec.Links {
					// virtctl for Windows for ARM 64 is still not shipped, avoid checking it
					// TODO: remove this once ready
					if strings.Contains(link.Href, "windows") && strings.Contains(link.Href, "arm64") {
						continue
					}

					g.Expect(link.Href).To(ContainSubstring(newCLIDLHost))
					res, err := httpClient.Head(link.Href)
					g.Expect(err).NotTo(HaveOccurred(), "HEAD failed for %s", link.Href)
					if res.Body != nil {
						g.Expect(res.Body.Close()).To(Succeed())
					}
					g.Expect(res.StatusCode).To(Equal(http.StatusOK), "non OK response for %s", link.Href)
				}
			}).WithContext(ctx).
				WithTimeout(60 * time.Second).
				WithPolling(5 * time.Second).
				Should(Succeed())

			By("check route")
			routeKey := client.ObjectKey{Name: downloadhost.CLIDownloadsServiceName, Namespace: tests.InstallNamespace}
			Eventually(func(g Gomega, ctx context.Context) {
				route := &v1.Route{}
				g.Expect(cli.Get(ctx, routeKey, route)).To(Succeed())
				g.Expect(route.Spec.Host).To(Equal(newCLIDLHost))
			}).WithContext(ctx).
				WithTimeout(60 * time.Second).
				WithPolling(time.Second).
				Should(Succeed())

			checkVirtioWinDownload(ctx, cli, httpClient, newCLIDLHost)
		})
	})
})

func getHTTPClient() *http.Client {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{Transport: customTransport, Timeout: time.Second * 3}
}

func checkVirtioWinDownload(ctx context.Context, cli client.Client, httpClient *http.Client, cliDLHost string) {
	GinkgoHelper()

	By("check the virtio-win iso download")
	var hcoPods corev1.PodList
	Expect(cli.List(ctx, &hcoPods, &client.MatchingLabels{
		"name": "hyperconverged-cluster-operator",
	})).To(Succeed())
	Expect(hcoPods.Items).ToNot(BeEmpty())

	pod := hcoPods.Items[0]
	idx := slices.IndexFunc(pod.Spec.Containers[0].Env, func(env corev1.EnvVar) bool {
		return env.Name == hcoutil.VirtIOWinDataFileEnvV
	})

	if idx < 0 {
		GinkgoLogr.Info(fmt.Sprintf("The HCO pod does not have the %s env var; virtio-win image download is not implemented", hcoutil.VirtIOWinDataFileEnvV))
		return
	}

	virtioWinDLPath := pod.Spec.Containers[0].Env[idx].Value

	virtioWinCM := &corev1.ConfigMap{}
	Expect(cli.Get(ctx, client.ObjectKey{Namespace: tests.InstallNamespace, Name: "virtio-win"}, virtioWinCM)).To(Succeed())

	virtioWinDLPathURL, urlExists := virtioWinCM.Data["virtio-win-image-download-url"]
	Expect(urlExists).To(BeTrue())
	Expect(virtioWinDLPathURL).To(ContainSubstring(cliDLHost))
	Expect(virtioWinDLPathURL).To(HaveSuffix(virtioWinDLPath))
	res, err := httpClient.Head(virtioWinDLPathURL)
	Expect(err).NotTo(HaveOccurred(), "HEAD failed for %s", virtioWinDLPathURL)
	if res.Body != nil {
		Expect(res.Body.Close()).To(Succeed())
	}
	Expect(res.StatusCode).To(Equal(http.StatusOK), "non OK response for %s", virtioWinDLPathURL)
}
