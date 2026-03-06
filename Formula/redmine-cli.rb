class RedmineCli < Formula
  desc "CLI for operating the Redmine REST API"
  homepage "https://github.com/YOUR_GITHUB_USER/redmine-cli"
  url "https://github.com/YOUR_GITHUB_USER/redmine-cli/archive/refs/heads/main.tar.gz"
  version "main"
  sha256 :no_check
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-trimpath", "-ldflags", "-s -w", "-o", bin/"redmine", "."
  end

  test do
    output = shell_output("#{bin}/redmine --help")
    assert_match "Redmine REST API", output
  end
end
