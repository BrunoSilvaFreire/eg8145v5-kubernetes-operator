build:
	podman build -t eg8145v5-ingress-operator:latest .

push-to-homelab: build
	podman push localhost/eg8145v5-ingress-operator:latest harbor.leroy.lab/library/eg8145v5-ingress-operator:latest

apply:
	kubectl apply -f k8s/deployment.yaml