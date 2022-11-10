module github.com/int128/kubectl-external-forward

go 1.16

require (
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.9
	github.com/google/wire v0.5.0
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/sync v0.1.0
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/cli-runtime v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/klog/v2 v2.60.1
)
