# Tailscale S3 bucket proxy 

Serve an S3 bucket on your tailnet, as a "relatively static" website.

Usage: 

``` shellsession
Usage of ./tailscale-s3-proxy:
  -bucket string
    	The S3 bucket to serve from. See -key-prefix if you need to serve part of it.
  -debug
    	Print out HTTP requests as they come in
  -hostname string
    	Tailscale hostname to serve on, used as the base name for MagicDNS or subdomain in your domain alias for HTTPS.
  -key-prefix string
    	Prefix for the keys in the bucket to serve
  -state-dir string
    	Alternate directory to use for Tailscale state storage. If empty, a default is used. (default "./")
  -use-https
    	Serve over HTTPS via your *.ts.net subdomain if enabled in Tailscale admin.
```

To use this, you're going to need a tailscale auth key, set in the `TS_AUTH_KEY` environment variable, _and_ your environment needs to be set up so that the AWS SDK for Go can find credentials. It's SDK v1, because SDKv2 isn't supported by the s3fs library (yet).
