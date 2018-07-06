Servers hosted on [OVH](https://www.ovh.ie) Cloud VPS:

* build servers.
* router and other control services (Kubernetes).

OVH is the only cloud provider that provides the best price/performance ration. Difference is dramatical, OVH server for 19$ is able to build snap in 1m43s vs 2m6s on DigitalOcean/Vultr/UpCloud 20$ server.

Only 20$+ servers are suitable for build — build job concurrency equals to cpuCount + 1. So, for 10$ server it means that there is a chance that second job will fail with out of memory (1 vCPU and 2 GB RAM).

DigitalOcean/Vultr/UpCloud has the same pricing and performance. 

If you don't need superior performance (or if you agree to pay twice more, 40$ instead of 20$ for build server) and high availability, DigitalOcean is the best choice. And Rancher supports it (no need to create nodes by hand).

But if you need high availability, use Vultr — because Vultr provides [BGP](https://www.vultr.com/docs/high-availability-on-vultr-with-floating-ip-and-bgp) and you can easily use [MetalLB](https://metallb.universe.tf) as a load balancer for free (OVH/DigitalOcean offers it for 20$). For build service high availability is not required because client retries connection attempts to router several times (yes, will be a problem if DNS server always returns the same server IP, but it is a tradeoff, OVH Cloud VPS is quite reliable) and router will not return unhealhy build agents.

## Scaleway

Scaleway maybe great for general purpose sites, but not as a build server because of Atom CPU. Another problem is that only Paris location is available (Amsterdam doesn't support new Start NVMe plans).

[Rancher](https://rancher.com) instance to manage Kubernetes is hosted on Scaleway and it works great (for €7.99 you get 4 Atom CPU and 4GB RAM).

## Vultr

In general, good. There are number of minor issues, that's make Vultr not so awesome compared to DigitalOcean:

* Snapshot per the whole disk, as result, very slow. At least 3 minutes is required to restore (25 GB SSD). So, this feature is not usable at all, and it is more suitable just create a new server from scratch using boot script (~45 seconds).

## OVH

VPS Cloud.

* No sexy UI (but UI is still fully functional and quite usable).
* No explicit ISO or iPXE support.
* ~3 minutes to install, then ~3 minutes to boot in Resque mode to install custom OS.
* No hourly billing.
* 100 Mbits/s.

But NVMe disks (and as result, superior performance), ability to install any OS using Resque mode. What else do you want?

## UpCloud

UpCloud is great. Really great. It is able even to gracefully shutdown server from admin panel.

## DigitalOcean

See above.

## Linode

Linode was good 5 years ago, but now no reason even try to use it and do benchmarks. Anyway, see why [Linode was rejected](https://github.com/develar/electron-build-service/issues/3#issuecomment-349280483).

## Benchmarks

Ok, Scaleway sucks, but maybe 14$ for 4 Atom CPU and 8 GB RAM is a good reason to overcome all issues (e.g. use Ubuntu instead of CoreOS, take a risk to use outdated and not tested Linux Kernel to be able reboot server)?
Well, to build AppImage (gzip, not CPU hungry) and deb (CPU hungry because of xz compression using 7z):
* Vultr: 81s 43ms (20$)
* OVH: 59s 668ms (VPS Cloud 2, 19$ or 17$ if pay per year)
* Scaleway: 111s 913ms (C2S)

Why? Because Atom CPU is slow compared to 1 vCPU on Vultr/UpCloud. Of course, xz must be not used because slow and badly implemented (not really multi-threaded), only 7z must be, but still. So, 4 Atom CPU for build task is not so good as 2 vCPU even if you use modern decent multi-cpu aware software like 7zip.

So, even if Vultr costs 20$ (not 14$) and offers 4 GB RAM instead of 8 GB RAM, Vultr/OVH is a winner.