class Shadow < Formula
  desc "AI agent memory layer — capture corrections, create persistent rules across all your coding tools"
  homepage "https://github.com/joevilcai666/shadow"
  url "https://github.com/joevilcai666/shadow/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "PLACEHOLDER_SHA256"
  license "MIT"
  head "https://github.com/joevilcai666/shadow.git", branch: "main"

  depends_on "go" => :build
  depends_on "node" => :build

  def install
    # Build web assets.
    cd "web" do
      system "npm", "install"
      system "npm", "run", "build"
    end

    # Build Go binary with embedded web assets.
    system "go", "build", *std_go_args(ldflags: "-s -w -X github.com/joevilcai666/shadow.Version=#{version}"), "./cmd/shadow/"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/shadow version")
  end
end
