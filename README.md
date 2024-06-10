# rdnbd (**R**ead **O**nly **N**etwork **B**lock **D**evice)


**rdnbd** allows you mount HTTP file directly!

```bash
$ go install github.com/agfn/rdnbd/cmd/rdnbd@latest
$ sudo rdnbd -device /dev/nbd0 -cache .cache https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.5.0-amd64-netinst.iso
$ lsblk /dev/nbd0
$ sudo mkdir -p /mnt/debian && sudo mount -o ro /dev/nbd0p1 /mnt/debian
```

**rdnbd** relies on the [Network Block Device](https://docs.kernel.org/admin-guide/blockdev/nbd.html). Ensure you have NBD installed:

```bash
$ ls -lh /dev/nbd*
brw-rw---- 1 root 6 43,   0 Jun 10 17:03 /dev/nbd0
...
```

If NBD is not installed, refer to the *Install NBD* section

## How it Works

**Prerequisites**

- The HTTP server must support [range requests](https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests), enabling partial file downloads.
- Running on Linux with the NBD driver.

The concept is straightforward: emulate a block device to mount it.

NBD (Network Block Device) is designed for this purpose. It requires a server to provide block data and a kernel driver to map block data to a block device `/dev/nbdx`.

With the help of [go-nbd](https://github.com/pojntfx/go-nbd), I quickly built the NBD server. However, it was extremely slow due to network latency when mounting, so I implemented an on-disk cache to reduce download frequency.

The cache consists of two files:
- `cache`: stores cached block data.
- `cache.idx`: stores the cached block index.

When reading a block, it checks whether the block is cached by accessing `idx = cache.idx[block_id]`.
If `idx > 0`, the block has been cached and can be read from `cache[(block_id - 1)]`. If the block is not cached, it downloads the block using HTTP range request.

## Install NBD

### Install NBD for WSL

1. Compile [WSL Kernel](https://github.com/microsoft/WSL2-Linux-Kernel) with `CONFIG_BLK_DEV_NBD=y`

    ```bash
    # kernel project is quite large, so clone the files you just need with `--depth 1`
    $ git clone --depth 1 -b linux-msft-wsl-5.15.153.1 https://github.com/microsoft/WSL2-Linux-Kernel
    $ cp Microsoft/config-wsl .config
    $ sed -i '/CONFIG_BLK_DEV_NBD/d' .config
    $ echo 'CONFIG_BLK_DEV_NBD=y' >> .config
    $ make -j12
    ```

2. Restart WSL with Compiled Kernel.

    Copy kernel to Windows file system:
    ```bash
    # copy to E:\wsl\kernel
    $ cp arch/x86/boot/bzImage /mnt/e/wsl/kernel
    ```
    Config WSL kernel. Edit `$env:UserProfile\.wslconfig` file and add the following line:
    ```toml
    [wsl2]
    kernel=E:\\wsl\\kernel
    ```
    Restart WSL:
    ```powershell
    $ wsl --shutdown
    $ wsl
    ```
