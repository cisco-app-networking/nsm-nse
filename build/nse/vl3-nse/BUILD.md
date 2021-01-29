# Building the virtual L3 NSE

1. To build the vL3 NSE, clone this repo and checkout this branch:

   ```bash
   $ mkdir -p $GOPATH/src/github.com/cisco-app-networking
   $ git clone https://github.com/cisco-app-networking/nsm-nse
   ```

1. Build the vL3 NSE:

   ```bash
   $ ORG=myuser TAG=foo make docker-vl3
   ```

   - The result is an image called `myorg/vl3_ucnf-nse:foo`
