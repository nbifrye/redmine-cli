class RedmineCli < Formula
  desc "CLI for interacting with the Redmine REST API"
  homepage "https://github.com/nbifrye/redmine-cli"
  head "https://github.com/nbifrye/redmine-cli.git", branch: "main"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./"
  end

  test do
    output = shell_output("#{bin}/redmine --help")
    assert_match "Redmine CLI", output
  end
end
