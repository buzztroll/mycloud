My Cloud
========

This program will echo to stdout the name of the cloud on which it is
running.  The following clouds are supported:

| Supported Cloud         | Output String |
|-------------------------|---------------|
| Amazon Web Services EC2 | AWS           |
| Google Compute Engine   | GCE           |
| Azure                   | Azure         |
| OpenStack               | OpenStack     |
| Digital Ocean           | DigitalOcean  |

If the cloud on which *mycloud* is run is not in the above list, or
the program fails to detect the cloud the string *UNKNOWN* is writen to
stdout and a non-zero exit code is returned.

Note: that *mycloud* can only detect Azure for linux systems when run as root.

Metadata Keys
-------------

On the following clouds *mycloud* can also pull specific metadata tags out
of the cloud's metadata server:

- AWS
- GCE
- OpenStack
- DigitalOcean

```{r, engine='bash'}
$ ./mycloud-Linux-x86_64 --key ami-id
AWS
ami-deadbeef
```

Download
--------

This program is writen in GO and thus there is a single executable binary.
It is available for download below for the list of architectures below:

[Linux 64 bit](https://s3-us-west-1.amazonaws.com/whatismycloud/mycloud-Linux-x86_64)
