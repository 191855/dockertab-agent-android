variable "VERSION" {
  default = "dev"
}

variable "REGISTRY" {
  default = "ghcr.io/191855/dockertab-agent-android"
}

group "default" {
  targets = ["agent-android"]
}

target "agent-android" {
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64", "linux/arm/v7"]
  args = {
    VERSION = VERSION
  }
  tags = [
    "${REGISTRY}:${VERSION}",
    "${REGISTRY}:latest",
  ]
}
