# Kubecredcache ⚡️

Kubecredcache makes many `kubectl` operations faster when used with AWS EKS.

When using EKS, many users have a Kubernetes configuration that runs either
`aws-iam-authenticator` or the `aws eks get-token` command. This means that,
every time the user runs `kubectl`, they also run one of those commands which
makes a blocking API call to AWS to generate a new authentication token. While
this authentication token is valid for 10 to 15 minutes, it is typically
discarded when `kubectl` finishes, and the next time the user runs `kubectl`
the process repeats. This is a lot of 🐢 overhead!

One of these utilities, `aws-iam-authenticator`, has a `--cache` option, but
it keys the cache based on the contents of the `AWS_PROFILE` environment
variable--if you're not using that to switch between clusters, you're out of
luck.

Kubecredcache wraps the call to `aws-iam-authenticator` or `aws eks get-token`
and caches the authentication token the first time `kubectl` is called.
The cached token is used on subsequent invocations of `kubectl`, saving lots
of time and making `kubectl` seem ⚡️peppier!

## How to Install Kubecredcache

### 🍎 Mac OS X

Install kubecredcache using Homebrew:

```bash
brew install jamesdobson/kubecredcache/kubecredcache
```

## Using Kubecredcache

To speed up your `kubectl` commands, edit your kubeconfig file to call
`kubecredcache`, passing the previous authentication command to it. In general,
the steps are:

1. Change `command` from `aws` or `aws-iam-authenticator` to `kubecredcache`.
2. Add whatever used to be in `command` to the beginning of the `args` list.

Here are examples for `aws` and `aws-iam-authenticator`.

### Wrapping the `aws` Command

When using the `aws` command, your kubeconfig will have a section that looks
like this:

```yaml
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: aws
      args:
      - --region
      - us-west-2
      - eks
      - get-token
      - --cluster-name
      - mycluster
```

To use `kubecredcache`, change `command` to `kubecredcache` and insert `aws`
at the beginning of the `args` list:

```yaml
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: kubecredcache
      args:
      - aws
      - --region
      - us-west-2
      - eks
      - get-token
      - --cluster-name
      - mycluster
```

### Wrapping the `aws-iam-authenticator` Command

When using the `aws-iam-authenticator` command, your kubeconfig will have a
section that looks like this:

```yaml
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: aws-iam-authenticator
      args:
        - "token"
        - "-i"
        - "mycluster"
```

To use `kubecredcache`, change `command` to `kubecredcache` and insert
`aws-iam-authenticator` at the beginning of the `args` list:

```yaml
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: kubecredcache
      args:
        - aws-iam-authenticator
        - "token"
        - "-i"
        - "mycluster"
```

## How it Works

Kubecredcache is aware that you might be connecting to different kubernetes
clusters and even the same cluster with different users. It handles these
cases by using the following as its cache key:

1. The cluster name/id - derived from the command line arguments to `aws` or
`aws-iam-authenticator`.
2. The region (if present) - also derived from the command line arguments.
3. The AWS access key ID - found in the `AWS_ACCESS_KEY_ID` environment
variable.

Kubecredcache stores the cached credentials in YAML files in its
`~/.kubecredcache` directory. The cache files are named according to the
cluster name/id: `cache-<CLUSTER_NAME>.yaml`. They contain a mapping from the
cache key to the cached credentials.

### 🗑 Garbage Collection

Every time kubecredcache runs, it scans the cache directory `~/.kubecredcache`
for cache files that haven't been written in more than 20 minutes (by default).
If any such files are found, they're deleted.

When kubecredcache writes a token to a cache file, it checks the tokens for
other cache keys in the file and removes them if they're expired.

### Appendix

The output of `aws` and `aws-iam-authenticator` looks something like this:

```json
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1alpha1",
  "spec": {},
  "status": {
    "expirationTimestamp": "2020-06-27T03:21:25Z",
    "token": "k8s-aws-v1.<<<LOTS_OF_BASE64>>>"
  }
}
```

Kubecredcache stores the entire JSON object in the cache as a mostly opaque
block, but it does interpret the `status.expirationTimestamp` value to
determine when the token expires.
