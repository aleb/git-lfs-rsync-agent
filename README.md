# Rsync Custom Transfer Agent for Git LFS

The rsync [git-lfs](https://git-lfs.github.com/) custom transfer agent allows
transferring the data through rsync, for example using SSH authentication.

Natively, git-lfs allows transferring data through HTTPS only.
This means to use git-lfs with your standard git repository,
you need a separate git-lfs server for storing the large files.
Unfortunately there is no official server implementation.

Luckily git-lfs has a Custom Transfer Adapter which allows using [Custom Transfer](https://github.com/git-lfs/git-lfs/blob/master/docs/custom-transfers.md)
Agents for transferring the data!

To use it, add to your `.lfsconfig`
[config file](https://github.com/git-lfs/git-lfs/blob/master/docs/man/git-lfs-config.5.ronn)
something like:
```
$ git config --replace-all lfs.concurrenttransfers 8
$ git config --replace-all lfs.standalonetransferagent rsync
$ git config --replace-all lfs.customtransfer.rsync.path git-lfs-rsync-agent
$ git config --replace-all lfs.customtransfer.rsync.concurrent true
$ git config --replace-all lfs.customtransfer.rsync.args SERVER:PATH
```

When pushing a branch to the origin repo, git-lfs tries to contact the inexistent
git-lfs server and dies because it can't. If this happens, make sure the locks
verify feature is disabled:
```
$ git config --replace-all lfs.locksverify false

```
