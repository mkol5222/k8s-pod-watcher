```shell

# feed server query POST
curl localhost:9090/pods --data '{"Label":"app=web2","Namespace":"d2"}' -vvv -H 'Content-Type: application/json'

# feed server query GET
curl 'localhost:9090/pods?namespace=d2&label=app%3Dweb2'
