# kubectl-socat

This is a kubectl plugin of TCP proxy.
You can connect to a remote host via port-forward and socat pod as follows:

```
kubectl socat
↓
kubectl port-forward
↓
Pod (socat)
↓
remote host:port
```

## Getting Started

### Setup

Install the latest release from [Homebrew](https://brew.sh/) or [GitHub Releases](https://github.com/int128/kubectl-socat/releases).

```sh
# Homebrew
brew tap int128/kubectl-socat https://github.com/int128/kubectl-socat
brew install kubectl-socat
```

### Connect

Run the following command:

```console
% kubectl socat -l 10000 -r www.example.com:80
I0305 16:59:09.797227   65840 external_forwarder.go:46] creating a socat pod with image ghcr.io/int128/kubectl-socat/mirror/alpine/socat:latest
I0305 16:59:10.671068   65840 external_forwarder.go:68] created pod default/socat-qn4zt
I0305 16:59:10.691330   65840 pod.go:29] pod default/socat-qn4zt is still Pending
I0305 16:59:11.270674   65840 pod.go:29] pod default/socat-qn4zt is still Pending
I0305 16:59:12.381635   65840 pod.go:29] pod default/socat-qn4zt is still Pending
I0305 16:59:13.723532   65840 pod.go:29] pod default/socat-qn4zt is still Pending
I0305 16:59:15.336288   65840 external_forwarder.go:107] starting port-forwarder from 10000 to default/socat-qn4zt:10000
I0305 16:59:15.375911   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:13 socat[1] W ioctl(5, IOCTL_VM_SOCKETS_GET_LOCAL_CID, ...): Not a tty
Forwarding from 127.0.0.1:10000 -> 10000
Handling connection for 10000
```

Now you can connect to `www.example.com:80` via a socat Pod from your laptop.

```console
% curl localhost:10000
<?xml version="1.0" encoding="iso-8859-1"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"
         "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
	<head>
		<title>404 - Not Found</title>
	</head>
	<body>
		<h1>404 - Not Found</h1>
	</body>
</html>
```

You can check connection by the socat log.

```console
I0305 16:59:24.792098   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:24 socat[1] N accepting connection from AF=2 127.0.0.1:42860 on AF=2 127.0.0.1:10000
I0305 16:59:24.794406   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:24 socat[1] N forked off child process 6
I0305 16:59:24.794740   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:24 socat[1] N listening on AF=2 0.0.0.0:10000
I0305 16:59:24.808676   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:24 socat[6] N opening connection to AF=2 93.184.216.34:80
I0305 16:59:24.932344   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:24 socat[6] N successfully connected from local address AF=2 10.10.10.10:56792
I0305 16:59:25.066358   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:25 socat[6] N socket 1 (fd 6) is at EOF
I0305 16:59:25.190113   65840 pod.go:55] default/socat-qn4zt: 2021/03/05 07:59:25 socat[6] N socket 2 (fd 5) is at EOF
```

Press ctrl-c to stop the command. It cleans up the socat pod.

```console
^CI0305 16:59:27.807956   65840 external_forwarder.go:80] deleting pod default/socat-qn4zt...
I0305 16:59:27.808186   65840 external_forwarder.go:118] stopped port-forwarder
I0305 16:59:27.867680   65840 external_forwarder.go:84] deleted pod default/socat-qn4zt
```


## Contributions

This is an open source software licensed under Apache License 2.0. Feel free to open issues and pull requests for improving code and documents.
