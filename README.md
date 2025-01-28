# Colony

## installation

```sh
# todo skip this section
go build .
wget ...
```

## use colony cli

### prerequisite 

check to see if colony is running with

```sh
docker ps -a

CONTAINER ID   IMAGE                      COMMAND                  CREATED          STATUS          PORTS     NAMES
2a4fda866f06   rancher/k3s:v1.30.2-k3s1   "/bin/k3s server --d…"   17 minutes ago   Up 17 minutes             colony-k3s
```

if a container is running run `./colony destroy` before proceeding. 

### running colony


to run the colony you'll need an api key. go to [colony.konstruct.io](https://colony.konstruct.io/) and sign in. visit the left nav bar and create a new api key. 

we use `enp4s0`  as the interface for smee to listen on (public vlan 1331)
select an ip address in the network for kube-vip to use to point to the tink stack for the load-balancer-ip

```sh
./colony init \
    --api-key a1a3cee8-2e3a-4792-8df1-aad1f66b4c3e \
    --load-balancer-interface enp4s0 \
    --load-balancer-ip 10.91.13.5

kubens tink-system

# patch the colony-agent image until we publish a new release
kubectl -n tink-system set image deployment/colony-colony-agent colony-agent=ghcr.io/konstructio/colony-agent:93fde8b
```

### auto discover machiens in your data center

```sh
./colony add-ipmi \
    --ip 10.90.14.13 \
    --username admin \
    --password $M3PASS \
    --auto-discover

# wait just a minute before discovering your next to avoid errors updating objects in cluster

./colony add-ipmi \
    --ip 10.90.14.14 \
    --username admin \
    --password $M4PASS \
    --auto-discover

# # only two machines are required, optionally add a 3rd
# ./colony add-ipmi \
#     --ip 10.90.14.15 \
#     --username admin \
#     --password $M5PASS \
#     --auto-discover
```

### create civo stack enterprise

```sh
./colony assets

# use the value in the IP column as the static IP address
# *note:* using these pre-assigned ip addresses until further testing is done
NAME               HOSTNAME                  IP            MAC                STATUS  
0c-42-a1-f2-dc-ed  discovered-not-inspected  10.91.13.240  0c:42:a1:f2:dc:ed
10-70-fd-e4-19-1b  discovered-not-inspected  10.91.13.243  10:70:fd:e4:19:1b  

gives you the ip address to use in the static $IP/24 field
```

#### visit colony.konstruct.io

visit [https://colony.konstruct.io/clusters](https://colony.konstruct.io/clusters) and create a new CivoStack.   

in the initial configuration enter the values
1. cluster name: eg. civo-cse
2. gateway ip: 10.91.13.254
   1. can be verified with `ip a show enp4s0` on the laptop vm
3. optionally add additional SANS and click Next
4. fill in the required values for all fields following validation 
   1. *note:* any values on step 2 are valid and no inputs are verified but will be passed through and added to a configmap in the new cluster
5. run `./colony assets` to get the ip addresses to use for the control plane
   1. the NAME column will match the user interface, provide the IP address column value $IP/24
6. click Create Civo Stack

#### wait for the CSE cluster to be healthy

download the kubeconfig from the clusters page and copy config to the laptop machine where you can access the 
cluster from the datacenter network

export KUBECONFIG=$downloaded-value


kubectl get nodes 

### the end

## cleaning up provisoined machines

step 1, wipe the disks on the machines (today its limited to the nvme* drives)

./colony assets

./colony deprovision --hardware-id 10-70-fd-e4-19-1b
./colony deprovision --hardware-id 0c-42-a1-f2-dc-ed

./colony destroy

delete all assets from the user interface before provisioning again, this will mark them deleted in the DB 

then start again from the top

#### OR

kubectl delete hw,wf,machines,j --all
kubectl delete secret 10-90-14-13 10-90-14-14

wait a minute or two 
before starting again, somet of the tinkerbell resources take a second to clean 
up and they are the first resources recreated. 
