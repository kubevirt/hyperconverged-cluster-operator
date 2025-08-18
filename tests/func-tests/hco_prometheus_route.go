package tests

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	openshiftroutev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hcoTempRouteName     = "hco-prom-route"
	webhookTempRouteName = "hco-wh-prom-route"
)

type HCOPrometheusClient struct {
	url   string
	token string
	cli   *http.Client
}

var (
	hcoClient     *HCOPrometheusClient
	hcoClientOnce = &sync.Once{}

	webhookClient     *HCOPrometheusClient
	webhookClientOnce = &sync.Once{}
)

func GetHCOPrometheusClient(ctx context.Context, cli client.Client) (*HCOPrometheusClient, error) {
	var err error
	hcoClientOnce.Do(func() {
		hcoClient, err = newHCOPrometheusClient(ctx, cli, "hco-bearer-auth", getOperatorTempRouteHost(hcoTempRouteName))
	})

	if err != nil {
		return nil, err
	}

	if hcoClient == nil {
		return nil, fmt.Errorf("HCO client wasn't initiated")
	}

	return hcoClient, nil
}

func GetWebhookPrometheusClient(ctx context.Context, cli client.Client) (*HCOPrometheusClient, error) {
	var err error
	webhookClientOnce.Do(func() {
		webhookClient, err = newHCOPrometheusClient(ctx, cli, "hco-webhook-bearer-auth", getOperatorTempRouteHost(webhookTempRouteName))
	})

	if err != nil {
		return nil, err
	}

	if webhookClient == nil {
		return nil, fmt.Errorf("HCO client wasn't initiated")
	}

	return webhookClient, nil
}

func newHCOPrometheusClient(ctx context.Context, cli client.Client, secretName string, getHostFn getTempRouteHostFunc) (*HCOPrometheusClient, error) {
	secret := &corev1.Secret{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: InstallNamespace, Name: secretName}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to read the secret %s; %w", secretName, err)
	}

	ticker := time.NewTicker(5 * time.Second)
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	for {
		select {
		case <-ticker.C:
			tempRouteHost, err := getHostFn(ctx, cli)
			if err != nil {
				continue
			}

			httpClient, err := rest.HTTPClientFor(GetClientConfig())
			if err != nil {
				return nil, fmt.Errorf("can't create HTTP client; %w", err)
			}

			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // This is needed for self-signed certificates in the test environment
				},
			}

			return &HCOPrometheusClient{
				url:   fmt.Sprintf("https://%s/metrics", tempRouteHost),
				token: string(secret.Data["token"]),
				cli:   httpClient,
			}, nil

		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for HCO Prometheus metrics route to be available")
		}
	}
}

func (hcoCli HCOPrometheusClient) GetHCOMetric(ctx context.Context, query string) (float64, error) {
	req, err := http.NewRequest(http.MethodGet, hcoCli.url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", string(hcoCli.token)))

	resp, err := hcoCli.cli.Do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to read the temp route status: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, query) {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				return 0, fmt.Errorf("metric line does not contain a value")
			}
			res, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil {
				return 0, fmt.Errorf("error converting %s to int: %v", line, err)
			}
			return res, nil
		}
	}
	return 0, nil
}

// CreateTempOperatorRoute creates a route to the HCO prometheus endpoint, to allow reading the metrics.
func CreateTempOperatorRoute(ctx context.Context, cli client.Client) error {
	return createTempRoute(ctx, cli, hcoTempRouteName, "kubevirt-hyperconverged-operator-metrics")
}

// CreateTempWebhookRoute creates a route to the HCO prometheus endpoint, to allow reading the metrics.
func CreateTempWebhookRoute(ctx context.Context, cli client.Client) error {
	return createTempRoute(ctx, cli, webhookTempRouteName, "hyperconverged-cluster-webhook-operator-metrics")
}

func createTempRoute(ctx context.Context, cli client.Client, routeName, serviceName string) error {
	err := openshiftroutev1.Install(cli.Scheme())
	if err != nil {
		return err
	}

	route := &openshiftroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: InstallNamespace,
		},
		Spec: openshiftroutev1.RouteSpec{
			Port: &openshiftroutev1.RoutePort{
				TargetPort: intstr.FromString("http-metrics"),
			},
			TLS: &openshiftroutev1.TLSConfig{
				Termination:                   openshiftroutev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: openshiftroutev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: openshiftroutev1.RouteTargetReference{
				Kind:   "Service",
				Name:   serviceName,
				Weight: ptr.To[int32](100),
			},
			WildcardPolicy: openshiftroutev1.WildcardPolicyNone,
		},
	}

	return cli.Create(ctx, route)
}

func DeleteTempOperatorRoute(ctx context.Context, cli client.Client) error {
	return deleteTempRoute(ctx, cli, hcoTempRouteName)
}

func DeleteTempWebhookRoute(ctx context.Context, cli client.Client) error {
	return deleteTempRoute(ctx, cli, webhookTempRouteName)
}

func deleteTempRoute(ctx context.Context, cli client.Client, routeName string) error {
	route := &openshiftroutev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: InstallNamespace,
		},
	}
	return cli.Delete(ctx, route)
}

type getTempRouteHostFunc func(context.Context, client.Client) (string, error)

func getOperatorTempRouteHost(routeName string) getTempRouteHostFunc {
	return func(ctx context.Context, cli client.Client) (string, error) {
		route := &openshiftroutev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      routeName,
				Namespace: InstallNamespace,
			},
		}
		err := cli.Get(ctx, client.ObjectKeyFromObject(route), route)
		if err != nil {
			return "", fmt.Errorf("failed to read the temp router; %w", err)
		}

		if len(route.Status.Ingress) == 0 {
			return "", fmt.Errorf("failed to read the temp route status")
		}

		return route.Status.Ingress[0].Host, nil
	}
}
