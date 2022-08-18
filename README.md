# tokenReviewSandbox

## Intro

This repo is a small sandbox for testing Kubernetes native authentication for a 
different project, and hopefully to give an example of how it may be used in
golang, as well as an example of gRPC golang authentication.

The gRPC interface is taken from the one in [google.golang.org/grpc/examples/helloworld/helloworld]()
as what this does is out of scope.

## Setup

This example assumes you have a Kubernetes server with full access 
(I used [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)).
The code assumes you're running in the default namespace.

First apply the serviceaccount and clusterrolebinding:
`kubectl apply -f yaml-files/service-accounts.yaml`

Then apply the server:
`kubectl apply -f yaml-files/server.yaml`

Then apply the client:
`kubectl apply -f yaml-files/client.yaml`

Anything interesting should be visible in the client and server logs:
- `kubectl logs token-client`
- `kubectl logs token-server`