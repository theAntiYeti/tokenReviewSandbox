package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	grpcAuth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
)

var (
	port           = flag.Int("port", 50051, "The server port")
	serviceAccount = flag.String("serviceaccount", "default:admin-user", "Name of the service account used by client")
)

type server struct {
	pb.UnimplementedGreeterServer
}

// GRPC definition taken from grpc example, as the actual API is out of scope for this example.
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	return &pb.HelloReply{Message: "Hello " + in.GetName()}, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.UnaryInterceptor(authenticateKubernetesToken),
	)

	pb.RegisterGreeterServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// Unary interceptor for validating token against issuing kubernetes cluster (for this prototype they're the same cluster)
func authenticateKubernetesToken(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	token, err := grpcAuth.AuthFromMD(ctx, "bearer")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing credentials")
	}

	// Now we have token we want to do a callback against the API
	// Get the URL to call
	log.Infof("Got to here")
	url, err := GetClusterUrl(token)
	if err != nil {
		return nil, err
	}
	var myUrl = url + "/apis/authentication.k8s.io/v1/tokenreviews"
	var data = fmt.Sprintf("{\"kind\":\"TokenReview\",\"apiVersion\":\"authentication.k8s.io/v1\",\"spec\":{\"token\":\"%s\"}}", token)

	log.Infof("Attempting to call %s", myUrl)
	verificationRequest, err := http.NewRequest("POST", myUrl, bytes.NewBuffer([]byte(data)))
	verificationRequest.Header.Add("Authorization", "Bearer "+token)
	verificationRequest.Header.Add("Content-Type", "application/json; charset=utf-8")

	reqDump, _ := httputil.DumpRequest(verificationRequest, true)
	log.Infof("Request to be sent: %s", reqDump)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Do(verificationRequest)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Infof("Returned response %s", body)

	status, err := parseAuthentication(body)
	if err != nil {
		return nil, err
	}
	if !status.Authenticated {
		return nil, fmt.Errorf("API response wasn't autenticated")
	}

	// Check for correct service account (current rule of thumb for authorisation)
	if status.User.Username != "system:serviceaccount:"+*serviceAccount {
		return nil, fmt.Errorf("user not authorised, not correct user")
	}

	return handler(ctx, req)
}

func parseAuthentication(body []byte) (*reviewStatus, error) {
	var uMbody reviewBody
	if err := json.Unmarshal(body, &uMbody); err != nil {
		return nil, err
	}
	log.Infof("Status is: %v", uMbody.Status)

	return &uMbody.Status, nil
}

type reviewUser struct {
	Username string `json:"username"`
}

type reviewStatus struct {
	Authenticated bool       `json:"authenticated"`
	User          reviewUser `json:"user"`
}

type reviewBody struct {
	Status reviewStatus `json:"status"`
}
