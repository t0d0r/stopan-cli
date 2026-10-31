class StopanCli < Formula
  desc "Command-line tool and terminal UI for the Stopan service desk"
  homepage "https://github.com/t0d0r/stopan-cli"
  url "https://github.com/t0d0r/stopan-cli/archive/refs/tags/v0.3.0.tar.gz"
  sha256 "abc763343d87f34afeafed24b94767b68c56d7f7e4cce73bb52b887c4622976b"
  license "BSD-2-Clause"
  head "https://github.com/t0d0r/stopan-cli.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"stopan", ldflags: "-s -w -X main.version=#{version}")
    generate_completions_from_executable(bin/"stopan", "completion")
  end

  test do
    assert_match "stopan #{version}", shell_output("#{bin}/stopan --version")
    assert_match "Usage: stopan", shell_output("#{bin}/stopan --help")
    assert_match "unknown command", shell_output("#{bin}/stopan nope 2>&1", 2)
    assert_match "complete -F _stopan stopan", shell_output("#{bin}/stopan completion bash")
  end
end
