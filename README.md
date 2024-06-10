# rdnbd (**R**ead **O**nly **N**etwork **B**lock **D**evice)


**rdnbd** allows you mount HTTP file directly!

```bash
$ go install github.com/agfn/rdnbd/cmd/rdnbd@latest
$ sudo rdnbd -device /dev/nbd0 -cache .cache https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.5.0-amd64-netinst.iso
$ sudo mkdir -p /mnt/debian && sudo mount /dev/nbd0 /mnt/debian
```

**rdnbd** relies on the [Network Block Device](https://docs.kernel.org/admin-guide/blockdev/nbd.html). Ensure you have NBD installed:

```bash
$ ls -lh /dev/nbd*
brw-rw---- 1 root 6 43,   0 Jun 10 17:03 /dev/nbd0
...
```

If NBD is not installed, refer to the *Install NBD* section

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
