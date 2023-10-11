# Arc

These are the components that store the shared training data.

When a user chooses to share images from their cameras, then this is the place
where they are stored. Once we have enough images, we can train a custom model
better suited to security cameras in general, or perhaps even tailored for
invidual needs.

# dev

### How to setup a development environment for the Arc server

-   Run `scripts/arc/compose` to run Postgres in a docker container
-   Run `scripts/arc/server` to start the Arc server
-   Change admin password to `12345678`:
    `curl -X POST -u admin:<password from above console logs> 'localhost:8081/api/auth/setPassword/1?password=12345678'`

### Connecting to gcloud

If you want to connect directly to google cloud from your dev environment:

-   Run `gcloud auth application-default login` (see
    https://cloud.google.com/docs/authentication/application-default-credentials)
