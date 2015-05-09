# Dockerfile for Heroku

This some tips for how to write `Dockerfile` for Heroku.

Start from minimal `Dockerfile`, 

```bash
$ heroku docker:init --template minimal
Wrote Dockerfile (minimal)
```

Prepare `Procfile`, you should not include environmental variable like `$PORT` in `Procfile`.
It would works on Heroku, but not on local environment (with `heroku docekr:start`).
Because `Procfile` is used for `docker run` command and environmental variable in `Procfile`
would extract from your host machine, not docker container.

To build & start your `Dockerfile`,

```bash
$ heroku docker:start
```

To debug your `Dockerfile`, just run & login to `heroku-docker-{hash}-start` image.
It is created after `docker:start` command. Check `${hash}` variables by `docker images`. 

```bash
$ docker run -it heroku-docker-${hash}-start /bin/bash
```

You need to prepare `/app/.procfile.d` in `Dockerfile`. And need to set root directory
this is same as `WORKDIR` of docker instruction. Set both.

Slug (`slug.tgz`) size must be under `300MB` ([https://devcenter.heroku.com/articles/slug-compiler#slug-size](https://devcenter.heroku.com/articles/slug-compiler#slug-size)). Check it,

```bash
$ docker run -it heroku-docker-${hash}-start /bin/bash
$ tar cfvz /tmp/slug.tgz -C / --exclude=.git --exclude=.heroku ./app
$ ls -lh /tmp/slug.tgz
```

## References

- []()
