Example tool to use containerd to pull from ACR registries using the Azure SDK for authenticating to an Azure Container Reigstry.
This example requires containerd >= 1.7 due to its use of the containerd transfer service, but older versions could be used by replacing the transfer service usage with `client.Pull`

### Usage

```console
$ az login --identity
$ go run . <name>.acurecr.io/<repo>:<tag>
# <output elided>
```

Replace `<name>` with the name of your ACR instance and `<repo>:<tag>` with the image/tag you want to pull from.
