# Building the pass-through NSE

1. To build the pass-through NSE, clone this repo and checkout this branch:

   ```bash
   $ mkdir -p $GOPATH/src/github.com/cisco-app-networking
   $ git clone https://github.com/cisco-app-networking/nsm-nse
   ```

1. Build the pass-through NSE:

   ```bash
   $ ORG=myorg TAG=foo make docker-pass-through
   ```

   - The result is an image called `myorg/pass-through-nse:foo`
