package main

import (
	"context"
	"flag"
	"fmt"
	grpcAuth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net"
	"os"
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
	token, err := grpcAuth.AuthFromMD(ctx, "kubernetesAuth")
	if err != nil {
		return nil, fmt.Errorf("couldn't parse auth header: %v", err)
	}

	// Pretend this came from a gRPC call :)
	fromFile, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}

	// Now we have token we want to do a callback against the API
	// Get the URL to call
	log.Infof("Got to here")
	url, err := GetClusterUrl(token)
	if err != nil {
		return nil, err
	}

	config := &rest.Config{
		Host:            url,
		BearerToken:     token,
		TLSClientConfig: rest.TLSClientConfig{CAData: fromFile},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Token Review Request made to the client's Kubernetes API.
	tr := authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := clientset.AuthenticationV1().TokenReviews().Create(ctx, &tr, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	if !result.Status.Authenticated {
		return nil, fmt.Errorf("user isn't who they say they are")
	}

	// Check for correct service account (current rule of thumb for authorisation)
	if result.Status.User.Username != "system:serviceaccount:"+*serviceAccount {
		return nil, fmt.Errorf("user not authorised, not correct user")
	}

	return handler(ctx, req)
}
