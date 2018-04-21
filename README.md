# cf-canary-router
Route requests based on metrics.

## Routing
The CF Canary Router is an application that a developer pushes that sits
infront of 2 versions of an application. It will slowly migrate HTTP requests
from the existing route to the canary route. It will determine success via the
provided [PromQL][prom-ql] query. If the query comes back with any results,
then it is deemed a success and the router will continue through the provided
plan.

The canary router will not receive all the requests, it will receive a
configurable percentage. This can be set via the `Plan`.

```
                     +---------------+        +-------------+
HTTP Requests -----> | Canary Router | -----> | Current App |
                     +---------------+        +-------------+
                             |
                             |                +------------+
                             +--------------> | Canary App |
                                              +------------+
```

**NOTE** that each request goes to ONLY a single route and is not copied to
both the `current app` and `canary app`.

## PromQL Queries
The canary router reads data from [Log Cache][log-cache] and applies the
PromQL to the given data. If the query yields a non-empty result, the canary
router considers it a success.

### Source IDs
Every metric in the query is required to set a label of `source_id`. If the metric is from/for an application, then the `source_id` will be its guid (`cf app <application-name> --guid`).

##### Example Queries

###### Any HTTP Requests
```
'http{source_id="e35ae4d8-849a-44e2-80b6-375b1fe4532d"}[1m]'
```
Simple query to see if an application has had any HTTP requests (emitted via
the [gorouter][gorouter]) within the last minute.

## Plans
Plans lay out how the canary router will route HTTP requests. A plan consists
of a series of steps. Each step is a percentage of requests to send to the
canary application and the length of time the step should take place. Once the
last step is surpassed, the canary router and the queries have yielded
success, all the traffic will be migrated to the canary application. If
however, the PromQL query determines a failure with the canary application, it
will fallback to the current application and route all requests there.

##### Example Plans

###### Single Step: 5m duration and 10% requests
```
{"Plan":[{"Percentage":10,"Duration":300000000000}]}
```

###### Two Steps: 5m duration and 10% requests, and 15m duration and 50% requests
```
{"Plan":[{"Percentage":10,"Duration":300000000000},{"Percentage":50,"Duration":900000000000}]}
```

**NOTE** Percentage must be an integer [0, 100].


## CF CLI Plug-in
The application is best installed with the CF CLI plug-in. It can be used via
the following:

```
cf canary-router --help
NAME:
   canary-router - Pushes a canary router

USAGE:
   canary-router

OPTIONS:
   -canary-app                The new app to start routing data to (REQUIRED)
   -current-app               The existing app to start routing data from (REQUIRED)
   -name                      Name for the canary router (defaults to 'canary-router')
   -username                  Username to use when pushing the app (REQUIRED)
   -force                     Skip warning prompt (default is false)
   -password                  Password to use when pushing the app (REQUIRED)
   -path                      Path to the canary-router app to push (defaults to downloading release from github)
   -plan                      The migration plan (defaults to '{"Plan":[{"Percentage":10,"Duration":300000000000}]}')
   -query                     The PromQL query that determines if the canary is successful (REQUIRED)
   -skip-ssl-validation       Whether to ignore certificate errors (default is false)
```

The plug-in will push and configure the canary router. It will also migrate
the routes over accordingly. After the plan has finished, the plug-in will
update the routes to either the canary application (success) or the current
application (failure). It will then delete the canary router.

[prom-ql]:   https://prometheus.io/docs/prometheus/latest/querying/basics/
[log-cache]: https://github.com/cloudfoundry/log-cache
[gorouter]:  https://github.com/cloudfoundry/gorouter
