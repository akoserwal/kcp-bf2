
# Steps to run

`export KUBECONFIG=.kcp/admin.kubeconfig`

`echo -n 'OFFLINE-TOKEN' | base64`

Copy the secret to offline-secret.yaml

kubectl apply -f offline-secret.yaml

`make install`

`make run`