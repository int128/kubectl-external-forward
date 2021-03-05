class KubectlSocat < Formula
  desc "kubectl plugin to connect to external host via Kubernetes cluster"
  homepage "https://github.com/int128/kubectl-socat"
  version "v0.1.0"

  on_macos do
    url "https://github.com/int128/kubectl-socat/releases/download/v0.1.0/kubectl-socat_darwin_amd64.zip"
    sha256 "a0862211648202ba80e3c33078ab1c8c2821dc6419e5fa8c496e83500cbfcd34"
  end
  on_linux do
    url "https://github.com/int128/kubectl-socat/releases/download/v0.1.0/kubectl-socat_linux_amd64.zip"
    sha256 "295b650da9766fc4e7ccc80201223f2366b800c419cc3bfab804a2d6e87f4cc0"
  end

  def install
    bin.install "kubectl-socat"
  end

  test do
    system "#{bin}/kubectl-socat -h"
  end
end
