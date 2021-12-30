## About

This example shows how to create a scheduled lambda function that will look for new HackerNews stories containing certain terms and send links to a slack channel.

## How it works

Here we are using the official HackerNews API which is described [here](https://github.com/HackerNews/API). We will also make use of the mantil KV store to persist data about items we have already processed.

When the function is invoked, it first fetches the ID of the [last processed item](api/alerts/alerts.go#L54) from the KV store and the ID of the [newest item](api/alerts/alerts.go#L61) from the HN API. For each item, it then checks if it contains certain terms. In this case, we are interested in stories that contain discussions about lambdas in go or general serverless topics. When such an item is found, we [traverse its parents](api/alerts/alerts.go#L83) until we find the associated story and send the link [to a slack channel](api/alerts/alerts.go#L134) via the provided webhook.

## Prerequisites

This example is created with Mantil. To download [Mantil CLI](https://github.com/mantil-io/mantil#installation) on Mac or Linux use Homebrew 
```
brew tap mantil-io/mantil
brew install mantil
```
or check [direct download links](https://github.com/mantil-io/mantil#installation).

To deploy this application you will need an [AWS account](https://aws.amazon.com/premiumsupport/knowledge-center/create-and-activate-aws-account/).

## Installation

To locally create a new project from this example run:
```
mantil new app --from hn-alerts
cd app
```

## Configuration 

Before deploying your application you will need to create a Slack webhook and add it as an environment variable for your function which will be used to post notifications to your Slack channel.

Detailed instructions on how to create a webhook can be found [here](https://slack.com/help/articles/115005265063-Incoming-webhooks-for-Slack).

Once your webhook is created you need to add URL to the `config/environment.yml` file as env variable for your function.
```
project:
  stages: 
    - name: development
      functions:
      - name: alerts
        cron: "* * * * ? *"
        env:
          SLACK_WEBHOOK: # add your slack webhook here
```

You can also change the function's schedule by changing the `cron` field. For example, this config will result in the function being invoked every 5 minutes:
```
project:
  stages: 
    - name: development
      functions:
      - name: alerts
        cron: "*/5 * * * ? *"
        env:
          SLACK_WEBHOOK: # add your slack webhook here
```

For more information refer to the [docs](https://github.com/mantil-io/mantil/blob/master/docs/api_configuration.md#scheduled-execution).

## Deploying the application

Note: If this is the first time you are using Mantil you will need to install Mantil Node on your AWS account. For detailed instructions please follow the [one-step setup](https://github.com/mantil-io/mantil/blob/master/docs/getting_started.md#setup)
```
mantil aws install
```
Then you can proceed with application deployment.
```
mantil deploy
```
This command will create a new stage for your project with the default name `development` and deploy it to your node.

The `alerts` function will be invoked every minute by default. You can also manually invoke it using `manil invoke alerts`.

## Cleanup

To remove the created stage from your AWS account destroy it with:
```
mantil stage destroy development
```

## Final thoughts

With this example you learned how to create a scheduled lambda function.

If you have any questions or comments on this template or would just like to share your view on Mantil contact us at [support@mantil.com](mailto:support@mantil.com).
