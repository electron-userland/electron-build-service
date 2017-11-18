provider "vultr" {
}

data "vultr_region" "frankfurt" {
  filter {
    name   = "name"
    values = ["Frankfurt"]
  }
}

data "vultr_plan" "m1024" {
  filter {
    name   = "price_per_month"
    values = ["5.00"]
  }

  filter {
    name   = "ram"
    values = ["1024"]
  }
}

data "vultr_os" "snapshot" {
  filter {
    name   = "family"
    values = ["snapshot"]
  }
}

data "vultr_snapshot" "master" {
  description_regex = "base"
}

// If the monitoring agent was installed on the original Droplet, the new Droplet will start collecting data and showing extended graphs automatically after a few minutes, regardless of whether or not Monitoring is selected now
// Our snapshot is created with monitoring. so, no need to specifiy monitoring option here
resource "vultr_instance" "electron-build-service" {
  name = "electron-build-service-green"
  hostname = "electron-build-service-green"
  region_id   = "${data.vultr_region.frankfurt.id}"
  plan_id     = "${data.vultr_plan.m1024.id}"
  os_id       = "${data.vultr_os.snapshot.id}"
  snapshot_id = "${data.vultr_snapshot.master.id}"
}