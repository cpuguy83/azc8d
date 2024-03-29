package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/transfer/image"
	"github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/containerd/v2/defaults"
)

func main() {
	defaultAddr := os.Getenv("CONTAINERD_ADDRESS")
	if defaultAddr == "" {
		defaultAddr = defaults.DefaultAddress
	}

	// For k8s, use the `k8s.io` namespace.
	// This example here just uses containerd's defaults similar to `ctr`.
	defaultNs := os.Getenv("CONTAINERD_NAMESPACE")
	if defaultNs == "" {
		defaultNs = "default"
	}

	addrFl := flag.String("address", defaultAddr, "address to containerd socket or pipe")
	nsFl := flag.String("namespace", defaultNs, "namespace to use for containerd objects")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Must provide an image to pull")
	}

	if err := do(ctx, *nsFl, *addrFl, flag.Arg(0)); err != nil {
		panic(err)
	}
}

func do(ctx context.Context, namespace, address, ref string) error {
	if namespace == "" {
		namespace = "default"
	}
	client, err := client.New(address, client.WithDefaultNamespace(namespace))
	if err != nil {
		return err
	}

	azid, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return err
	}

	from := registry.NewOCIRegistry(ref, http.Header{}, &azCredential{azid})
	to := image.NewStore(ref)

	if err := client.Transfer(ctx, from, to, transfer.WithProgress(progress)); err != nil {
		return err
	}
	return nil
}

func progress(p transfer.Progress) {
	fmt.Println(p.Event, p.Name)
}

type azCredential struct {
	id *azidentity.DefaultAzureCredential
}

func (az *azCredential) GetCredentials(ctx context.Context, ref, host string) (registry.Credentials, error) {
	if !strings.HasSuffix(host, ".azurecr.io") {
		// Not an ACR registry
		return registry.Credentials{}, nil
	}

	token, err := az.id.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return registry.Credentials{}, err
	}

	formData := url.Values{
		"grant_type":   {"access_token"},
		"service":      {host},
		"access_token": {token.Token},
	}

	resp, err := http.PostForm(fmt.Sprintf("https://%s/oauth2/exchange", host), formData)
	if err != nil {
		return registry.Credentials{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return registry.Credentials{}, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var auth acrAuthResp
	rdr := io.LimitReader(resp.Body, 1<<20)

	dt, err := io.ReadAll(rdr)
	if err != nil {
		return registry.Credentials{}, err
	}

	if err := json.Unmarshal(dt, &auth); err != nil {
		return registry.Credentials{}, err
	}

	return registry.Credentials{
		Username: "00000000-0000-0000-0000-000000000000",
		Secret:   auth.RefreshToken,
	}, nil
}

type acrAuthResp struct {
	RefreshToken string `json:"refresh_token"`
}
