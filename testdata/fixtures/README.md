# Failure fixtures

Each manifest breaks a cluster in exactly one way. Apply one, run `verdict`,
and confirm it reports exactly one finding of the expected rule.

    kubectl create namespace demo
    kubectl apply -f testdata/fixtures/c1-missing-configmap.yaml
    ./verdict

Clean up with `kubectl delete -f <file>` between fixtures.
