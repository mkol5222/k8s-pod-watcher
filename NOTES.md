```shell

# feed server query POST
curl localhost:9090/pods --data '{"Label":"app=web2","Namespace":"d2"}' -vvv -H 'Content-Type: application/json'

# feed server query GET
curl 'localhost:9090/pods?namespace=d2&label=app%3Dweb2'

curl 'localhost:9090/pods?namespace=default&label=app%3Dwebik' -s | jq -r '.[].ip-address'

# static build
CGO_ENABLED=0 go build

cpwd_admin start -name k8swatch2  -path /home/admin/k8s-pod-watcher -command "/home/admin/k8s-pod-watcher" -env KUBECONFIG=/home/admin/k3s

