# Homebrew formula for ayame-diff (macOS).
#
# The sha256 values below are placeholders; release automation replaces them
# with the checksums published in the release's SHA256SUMS for each tag.
class AyameDiff < Formula
  desc "Fast CSV/TSV key-diff and text-diff CLI with a local web GUI"
  homepage "https://github.com/hjosugi/ayame-diff"
  version "0.5.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # release automation fills this in
    end
    on_intel do
      url "https://github.com/hjosugi/ayame-diff/releases/download/v#{version}/ayame-diff-v#{version}-darwin-amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000" # release automation fills this in
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
