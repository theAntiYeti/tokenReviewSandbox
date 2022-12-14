package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
)

var (
	addr = flag.String("addr", "localhost:50051", "the address to connect to")
)

func main() {
	flag.Parse()

	// Get a TokenCredentials to authenticate to the server.
	token, err := getAuthenticationToken(context.Background())
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Create connection with the server.
	conn, err := grpc.Dial(*addr, grpc.WithPerRPCCredentials(token), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// Create a client for the gRPC interface and make a request.
	c := pb.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: "a dummy name"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}

	// Log the output.
	log.Printf("Greeting: %s", r)

	// Sleep a long time so
	time.Sleep(1000000 * time.Hour)
}

// getAuthenticationToken returns a TokenCredentials with bearer token from Kubernetes.
func getAuthenticationToken(ctx context.Context) (*TokenCredentials, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	exp := int64(3600)

	// Token request with 3600 seconds = 1h expiry.
	tr := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: &exp,
		},
	}

	result, err := clientset.CoreV1().ServiceAccounts("default").
		CreateToken(ctx, "admin-user", &tr, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	token := result.Status.Token

	return &TokenCredentials{token: token}, nil
}

// createAuthenticationToken gets a TokenCredentials with bearer token from Kubernetes to send to the server.
// This function is legacy as this is more reliably implementable with the Go Kubernetes client.
func createAuthenticationToken() (*TokenCredentials, error) {
	// Get the main service account token for getting a tmp token, do not send this to server
	log.Info("Getting service account token")
	kubernetesToken, err := getKubernetesToken()
	if err != nil {
		return nil, fmt.Errorf("error getting service account token: %v", err)
	}
	log.Infof("Service account token: %v", kubernetesToken)

	var bearer = "Bearer " + kubernetesToken
	var myUrl = "https://kubernetes.default.svc/api/v1/namespaces/default/serviceaccounts/admin-user/token"

	log.Infof("Attempting to call %s", myUrl)
	req, err := http.NewRequest("POST", myUrl, bytes.NewBuffer([]byte("{}")))
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting temp token: %v", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	token, err := parseTokenString(body)

	log.Infof("Token: %s", token)

	return &TokenCredentials{token: token}, err
}

// getKubernetesToken returns the service account token from the standard environment variable if present or
// the standard filepath if not.
func getKubernetesToken() (string, error) {
	fromEnv := os.Getenv("K8S_SERVICEACCOUNT_TOKEN")
	if fromEnv != "" {
		return fromEnv, nil
	}

	fromFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return "", err
	}

	return string(fromFile), nil
}

// parseTokenString unmarshals the header payload to extract the JWT token.
func parseTokenString(body []byte) (string, error) {
	type unmarshalStatus struct {
		Token string `json:"token"`
	}

	type unmarshalAll struct {
		Status unmarshalStatus `json:"status"`
	}

	var statusRes unmarshalAll
	if err := json.Unmarshal(body, &statusRes); err != nil {
		return "", fmt.Errorf("could not deserialise token: %v", err)
	}
	log.Infof("Status is: %v", statusRes.Status)

	return statusRes.Status.Token, nil
}

// TokenCredentials is an object that implements the PerRPCCredentials interface.
type TokenCredentials struct {
	token string
}

func (c *TokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "KubernetesAuth " + c.token,
	}, nil
}

func (c *TokenCredentials) RequireTransportSecurity() bool {
	return false
}
