class StopanCli < Formula
  desc "Command-line tool and terminal UI for the Stopan service desk"
  homepage "https://github.com/t0d0r/stopan-cli"
  url "https://github.com/t0d0r/stopan-cli/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "e1ee2d19313b316101ba566d362e49ac94a803a2c5eadfd339e34ccaf7b09364"
  license "BSD-2-Clause"
  head "https://github.com/t0d0r/stopan-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"stopan", ldflags: "-s -w -X main.version=#{version}")
  end

  test do
    assert_match "stopan #{version}", shell_output("#{bin}/stopan --version")
    assert_match "Usage: stopan", shell_output("#{bin}/stopan --help")
    assert_match "unknown command", shell_output("#{bin}/stopan nope 2>&1", 2)
  end
end
