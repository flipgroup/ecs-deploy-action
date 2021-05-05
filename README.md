# ecs-deploy-action

What do you mean you don't know what I am? I'm a GitHub Action that deploys to your AWS Elastic Container Service (ECS) clusters.

Hmm, let me try and clear this up. If you chuck some YAML in your workflow that looks like this:

```yaml
      - uses: flipgroup/ecs-deploy-action@v1
        with:
          cluster: default
          service: YourServiceName
          image-overrides: YourContainerName=111111111111.dkr.ecr.ap-southeast-2.amazonaws.com/Repo/YourImageName:${{ github.run_number }}
```

Then I will go and update your service with a new Docker image for you. That easy.

What about environment variables you ask? I'll just copy it over for you and assume you manage that some other way.

I also play nice with [aws-actions/configure-aws-credentials](https://github.com/aws-actions/configure-aws-credentials).
