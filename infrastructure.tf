variable "do_api_key" {}
variable "do_image_id" {}
variable "ssh_fingerprint" {}

provider "digitalocean" {
  token = "${var.do_api_key}"
}

// If the monitoring agent was installed on the original Droplet, the new Droplet will start collecting data and showing extended graphs automatically after a few minutes, regardless of whether or not Monitoring is selected now
// Our snapshot is created with monitoring. so, no need to specifiy monitoring option here
resource "digitalocean_droplet" "electron-build-service" {
  image = "${var.do_image_id}}"
  name = "electron-build-service-green"
  region = "fra1"
  size = "512mb"
  ssh_keys = ["${var.ssh_fingerprint}"]
}