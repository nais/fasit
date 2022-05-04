package local

import "k8s.io/client-go/rest"

func RESTConfig() *rest.Config {
	return &rest.Config{
		Host:        "http://localhost:6969",
		BearerToken: "i am cool",
		TLSClientConfig: rest.TLSClientConfig{
			CAFile: "unsecure-certificate.pem",
		},
	}
}
