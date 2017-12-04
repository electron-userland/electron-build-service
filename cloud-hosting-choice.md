Servers hosted on Vultr.

## Scaleway

Scaleway simply sucks. 

* No way to install custom OS (no iPXE or ISO support), prebuilt images are outdated, poor choice anyway (no not only RancherOS, but also CoreOS).
* And company doesn't respond to answers and comments. There are number of public repositories, but not [maintained](https://github.com/scaleway-community/scaleway-coreos/issues/1#issuecomment-347016327) and outdated.
* A lot of pitfalls and critical issues [without solutions](https://github.com/scaleway/image-ubuntu/issues/87) for months.

## Vultr

In general, good. There are number of minor issues, that's make Vultr not so awesome compared to DigitalOcean:

* Snapshot per the whole disk, as result, very slow. At least 3 minutes is required to restore (25 GB SSD). So, this feature is not usable at all, and it is more suitable just create a new server from scratch using boot script (~45 seconds).
* Not easy to install CoreOS because at least 2 GB RAM is required, and provided image is outdated. Solution? Just install provided image and upgrade (adds ~10 seconds to setup new server).

## UpCloud

UpCloud is great. Really great. It is able even to gracefully shutdown server from admin panel.

The only issue why it is not a winner — price. For 20$ on Vultr you will get 4 GB RAM. On UpCloud only 2 GB. Because of lzma compression, for one build task more than 1 GB RAM is required. So, UpCloud server will be twice as expensive (40$).

## DigitalOcean

Well... CPU as Vultr, price and RAM as UpCloud. So, no reason to use it. Benchmark "build AppImage and deb" was not performed, because results of "build AppImage" is enough to say that UpCloud is a winner (again, Vultr offers you twice more memory for the same price).

## Benchmarks

Ok, Scaleway sucks, but maybe 14$ for 4 Atom CPU and 8 GB RAM is a good reasons to overcome all issues (e.g. use Ubuntu instead of CoreOS)?
Well, to build AppImage (gzip, not CPU hungry) and deb (CPU hungry because of xz compression using 7z):
* Vultr: 81s 43ms
* Scaleway: 111s 913ms

Why? Because Atom CPU is slow compared to 1 vCPU on Vultr/Upcloud. Of course, xz must be not used because slow and not badly implemented (not really multi-threaded), only 7z must be, but still. So, 4 Atom CPU for build task is not so good as 2 vCPU.

So, even if Vultr costs 20$ (not 14$) and offers 4 GB RAM instead of 8 GB RAM, Vultr is a winner.