## Deploy Homer-app

helm repo add djjudas21 https://djjudas21.github.io/charts/
helm repo update djjudas21

helm -n homer-app install homer-app djjudas21/homer -f homer-app/homer-helm-values.yaml --create-namespace 

kubectl -n homer-app get pods

kubectl -n homer-app apply -f homer-app/ingress-route.yaml 

kubectl -n homer-app apply -f rancher-master/rancher-tls-secret.yaml 

