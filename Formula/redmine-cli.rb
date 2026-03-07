class RedmineCli < Formula
  desc "CLI for interacting with the Redmine REST API"
  homepage "https://github.com/nbifrye/redmine-cli"
  license "MIT"

  # GoReleaser がリリース時にこのブロックを自動更新する
  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/nbifrye/redmine-cli/releases/download/v0.1.0/redmine-cli_0.1.0_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/nbifrye/redmine-cli/releases/download/v0.1.0/redmine-cli_0.1.0_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/nbifrye/redmine-cli/releases/download/v0.1.0/redmine-cli_0.1.0_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/nbifrye/redmine-cli/releases/download/v0.1.0/redmine-cli_0.1.0_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  # 開発版は --HEAD でインストール可能
  head "https://github.com/nbifrye/redmine-cli.git", branch: "main"

  depends_on "go" => :build if build.head?

  def install
    if build.head?
      system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"redmine"), "./"
    else
      bin.install "redmine"
    end
  end

  test do
    system "#{bin}/redmine", "--help"
  end
end
