# kubectl-socat

This is a kubectl plugin of TCP proxy.
You can connect to a remote host via port-forward and socat Pod as follows:

```
kubectl socat
↓
kubectl port-forward
↓
Pod (socat)
↓
remote host:port
```
