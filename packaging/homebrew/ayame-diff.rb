class AyameDiff < Formula
  desc "Fast CSV, text, folder, and binary diff with a local web GUI"
  homepage "https://github.com/hjosugi/ayame-diff"
  version "0.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_intel do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  livecheck do
    url "https://github.com/hjosugi/ayame-diff"
    strategy :github_latest
  end

  def install
    bin.install "ayame-diff"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ayame-diff --version")
  end
end
