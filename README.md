# DoveRunner Forensic Watermark CDN Embedder Sample for Google Cloud Service Extensions

> **A reference implementation.** Fork this sample and adapt it to your own delivery setup — the
> plugin you deploy is yours to operate.
>
> - **Docs**: https://docs.doverunner.com/content-security/forensic-watermarking/embedding/
> - **Support**: https://doverunner.zendesk.com
> - **Security**: report a vulnerability through support
> - **Copyright**: ⓒ 2026 DoveRunner. All Rights Reserved. (doverunner.com)

This project serves as a sample implementation for embedding forensic watermarks using
**Google Cloud Service Extensions**. It is a Proxy-Wasm plugin written in Go. Attached to a global
external Application Load Balancer as an *edge extension*, it runs on every request that matches the
extension's match condition, **before the Cloud CDN cache lookup**: it resolves the watermark token in the request path, rewrites the path to
the rendition to serve, and answers `403` itself when the token is rejected. It does not store or
serve content — Cloud CDN fetches the rewritten path from your origin, a private Cloud Storage bucket.

For more information, refer to the [Service Extensions documentation](https://docs.cloud.google.com/service-extensions/docs/overview).

## Configuration
### Configure config.json
The plugin reads its settings from the *plugin configuration* you pass when you create the plugin
version (`--plugin-config-file`). Nothing is compiled in, so one build serves every deployment.
Update `config.json` with your specific settings:

```json
{
    "aesKey": "{Your DoveRunner Site Key}",
    "type": "unlabeled_a_variant",
    "availableInterval": 60,
    "prefixFolder": ["wm-contents"],
    "wmtPublicKey": "{Your DoveRunner Akamai Public Key}",
    "wmtPassword": "{Your DoveRunner Akamai Key Password}"
}
```

Line breaks inside the PEM public key are written as `\n`, as JSON requires.

`config.json` holds secrets: the site key lets anyone issue valid watermark tokens. Do not commit your
filled-in copy; pass it only with `--plugin-config-file`. Each plugin version stores its configuration,
so limit who can read the plugin in your project.

**wmt_type**: Indicates whether the issued token format is AES-encrypted or JWT. This corresponds to the specification provided when requesting the watermark token.
- [Watermark Token Request Specification](https://docs.doverunner.com/content-security/forensic-watermarking/embedding/session-manager/#api-data-json-format)

| Key                 | wmt_type | Description                                                                                                                                                                                                                                         |
|:--------------------|:---------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `aesKey`            | aes      | The Site Key issued by the DoveRunner ContentSecurity Service.                                                                                                                                                                                      |
| `type`              | aes, jwt | Required. The [Watermark File Folder Structure](#watermark-file-folder-structure): `unlabeled_a_variant`, `directory_prefix` or `aws`. The Akamai WMT route supports only `unlabeled_a_variant`.                                                      |
| `availableInterval` | aes      | Token validity window in **minutes** (0 disables the check).                                                                                                                                                                                                |
| `prefixFolder`      | aes, jwt | The top-level folder containing watermark files. Corresponds to `prefix_folder` in the [Watermark Token Request Specification](https://docs.doverunner.com/content-security/forensic-watermarking/embedding/session-manager/#api-data-json-format). |
| `wmtPublicKey`      | jwt      | PEM RSA public key for Akamai WMT (`BEGIN PUBLIC KEY` or `BEGIN RSA PUBLIC KEY`).                                                                                                                                                                   |
| `wmtPassword`       | jwt      | Password for WMT token generation/verification.                                                                                                                                                                                                     |

## Watermark Forms and Token Payload

The DoveRunner AES watermark token carried in the request path decrypts to a JSON payload.
Its fields drive every rewrite decision the embedder makes:

| Field                  | Description                                                                                     |
|:-----------------------|:------------------------------------------------------------------------------------------------|
| `watermark_data`       | Base64 of the finished `0`/`1` symbol pattern.                                                  |
| `streaming_format`     | `dash` or `hls`. Informational — it no longer selects the first segment.                        |
| `first_segment_number` | Segment number the pattern is aligned to. Required — a token without it is rejected with `403`. |
| `timestamp`            | Issue time, `YYYY-MM-DDThh:mm:ssZ`, checked against `availableInterval`.                        |
| `revoke_flag`          | Whether a revoke token must accompany the request.                                              |
| `sync_skip_bit`        | Sync offset added to the segment number; `-1` when inactive. `0` is a valid, active offset.     |
| `renditions`           | Physical rendition count, `2` or `4`. Absent means `2`.                                         |
| `lead_in`              | Unmarked lead-in before the first marked segment, in symbols. Absent means `4`; `0` is valid.   |

The last two fields select the **watermark form**. A Long-Form token may omit them — the
defaults reproduce the Long-Form constants:

| Form       | `renditions` | `lead_in` |
|:-----------|-------------:|----------:|
| Long-Form  |            2 |         4 |
| Short-Form |            2 |         0 |
| Micro-Form |            4 |         0 |

**How symbols map to segments.** One symbol spans one media segment. One segment carries
`log2(renditions)` bits of the pattern: 1 bit with two renditions, 2 bits with four. The first
`lead_in` symbols play unmarked; after them the pattern repeats for the rest of the stream.

**How symbols name renditions.** Under `unlabeled_a_variant` the symbol becomes a file name
prefix; under `directory_prefix` and `aws` it becomes a path segment:

| Symbol | `unlabeled_a_variant` prefix       | `directory_prefix` / `aws` segment |
|:------:|:-----------------------------------|:-----------------------------------|
| `0`    | *(none — the unlabeled A variant)* | `/0`                               |
| `1`    | `b.`                               | `/1`                               |
| `2`    | `c.` — four renditions only        | `/2`                               |
| `3`    | `d.` — four renditions only        | `/3`                               |

Direct requests for a marked variant file are refused with `403` according to the rendition
count: two renditions block the `b.` prefix, four block `b.`, `c.` and `d.`.

The Akamai WMT route is different: that token carries only the symbol pattern, and the sample
resolves it with fixed Long-Form settings (`renditions` 2) and no lead-in.

## Watermark File Folder Structure

### directory_prefix

- Distinguishes renditions by a `0`–`3` directory segment.

#### Example

```text
/wm-contents/output_path/cid/{0/1}/dash/stream.mpd
/wm-contents/output_path/cid/{0/1}/dash/video/0/seg-125.m4s
/wm-contents/output_path/cid/{0/1}/hls/master.m3u8
/wm-contents/output_path/cid/{0/1}/hls/video/0/0/stream.m3u8
/wm-contents/output_path/cid/{0/1}/hls/video/0/0/segment-125.ts
```

### unlabeled_a_variant

- A/B files exist in the same folder and are distinguished by filename prefixes.
- With two renditions the marked file is prefixed `b.`; with four renditions the
  prefixes are `b.`, `c.` and `d.`.

#### Example

```text
/wm-contents/output_path/cid/dash/stream.mpd
/wm-contents/output_path/cid/dash/video/0/seg-125.m4s
/wm-contents/output_path/cid/dash/video/0/b.seg-125.m4s
/wm-contents/output_path/cid/hls/master.m3u8
/wm-contents/output_path/cid/hls/video/0/0/stream.m3u8
/wm-contents/output_path/cid/hls/video/0/0/segment-125.ts
/wm-contents/output_path/cid/hls/video/0/0/b.segment-125.ts
```

### aws

- For content packaged by AWS Elemental MediaConvert. The `0`/`1` rendition directory sits
  directly above the streaming-format directory:

```text
/wm-contents/output_path/{0/1}/dash/stream.mpd
/wm-contents/output_path/{0/1}/hls/master.m3u8
```

The packager must encode the content twice — once into the `0` (unmarked) destination and once
into the `1` (marked) destination — name the manifest `stream` for DASH and `master` for HLS, and
use a GOP size of 60.

## Packaging Preconditions

The embedder picks a rendition from the **request path alone**. A path it cannot read is passed
through unmarked — playback still succeeds, but those segments carry no fingerprint and cannot be
traced. Your packager output must satisfy every rule below.

1. **End every media segment name with its segment number.** The number is taken after the last
   `_` or `-`, whichever comes later in the name, and everything following that separator must
   be digits. A name that adds a suffix after the number cannot be read and is served unmarked.
   Good `segment-125.ts`, `video_1080p-9.ts` · Bad `segment-125_backup.ts`

2. **End initialisation segment names with a lowercase `_init`, never with a number.** `_init` is
   the only spelling every `type` layout accepts. An init file whose name ends in a digit is read
   as a media segment instead, and whenever the symbol for that number is not `0` it is rewritten
   to a rendition path that does not exist and answers `404`.
   Good `video_init.mp4` · Bad `init-9.m4s`

3. **Keep the content path below the token deep enough.** Under `directory_prefix` the rendition
   directory is inserted 5 levels above `.ts`, `.m4s`, `.mp4`, `.aac`, `stream.m3u8` and
   `iframes.m3u8`, 4 levels above `subtitle.m3u8`, `.vtt` and any `.m4s`/`.mp4` one directory below a
   `subtitle` directory (`subtitle/ko/seg-3.m4s`; a segment placed directly in `subtitle/` is read as
   a media segment), and 2 levels above `.mpd` and any `.m3u8` whose name contains `master`;
   under `aws` it is always 2 levels. The path must hold at least that many parts,
   counting the file name — a shallower path receives no rendition directory at all.
   `unlabeled_a_variant` is unaffected, because it marks the file name rather than the path.
   Good `/{token}/out/cid/hls/video/avc1/1/segment-125.ts` · Bad `/{token}/cid/hls/segment-125.ts`

4. **Name HLS playlists as the layout expects.** Under `directory_prefix` an `.m3u8` name is
   recognised only when it contains `master`, or is exactly `stream.m3u8`, `iframes.m3u8` or
   `subtitle.m3u8`; the match is case-sensitive. Any other name falls back to a path that inserts
   `/0` above **every** `dash`, `hls` or `cmaf` directory in the path, and inserts nothing at all
   when the path contains none of those. `.mpd` manifests may carry any name.
   Good `master.m3u8`, `stream.m3u8` · Bad `index.m3u8`, `playlist.m3u8`

5. **Package to discrete segment files.** The rendition is chosen from the file name alone; no
   part of that decision looks at the `Range` header. An HLS `#EXT-X-BYTERANGE` layout — one file
   addressed by byte offsets — therefore presents a single name, and a single rendition, for the
   whole stream. Such a stream carries no usable fingerprint.
   Good `segment-1.ts`, `segment-2.ts`, … · Bad one `main.ts` plus `#EXT-X-BYTERANGE`

## Usage

The plugin intercepts client requests and routes them based on the URL structure:

1.  **Akamai WMT**: If the path starts with `wmt:`, it uses `wmt/akamai_wmt.go` to verify the token and rewrite the path.
2.  **DoveRunner AES**: If the path matches one of the `prefixFolder` entries, it uses `wmt/dove_runner_aes.go` to decrypt the watermark data and generate the appropriate content path.

Any other path passes through unchanged.

Keep content served through the Akamai WMT route under one of the `prefixFolder` folders too (for
example `/wmt:{token}/wm-contents/...`). A direct request for a marked variant file is refused only
when its path starts with a `prefixFolder` folder; content outside every `prefixFolder` is served to
anyone who knows its path.

**Token lifetime on the Akamai WMT route.** The sample checks only the RS256 signature of a
`wmt:` token — it enforces no expiry, and `availableInterval` applies to the DoveRunner AES route
only. Bound the URL or token lifetime upstream, for example with Akamai edge token authorization
or short-lived signed URLs: if nothing upstream bounds it, a leaked `wmt:` URL stays valid until
the signing key is rotated.

## CDN caching

The plugin handles the `REQUEST_HEADERS` event of an edge extension. Edge extensions run before the
URL map and before the Cloud CDN cache lookup, so the `:path` the plugin writes is what Cloud CDN
looks up and what it fetches from the origin. Because the rewritten path carries no token, viewers
that resolve to the same rendition of the same segment share one cache entry. The token itself is
validated on every request the extension matches, so an expired token is refused even when the
segment is cached.

A rejected token gets a `403` from the plugin; the request never reaches Cloud CDN or the origin.
The response body is a short reason, for example `Invalid watermark token`.

Keep the origin bucket **private**: a public bucket can be read at `storage.googleapis.com` without
the plugin, marked variant files included. Only the load balancer may read it (see
[Prerequisites](#prerequisites)). The load balancer can read every object, so the edge extension's
match condition decides what is protected: a request it does not match is served as is.

**Guidance**
- Create the backend bucket with Cloud CDN in `FORCE_CACHE_ALL` mode; objects in a private bucket are
  cached only in that mode:
  `gcloud compute backend-buckets create BACKEND_BUCKET --gcs-bucket-name=BUCKET_NAME --enable-cdn --cache-mode=FORCE_CACHE_ALL --default-ttl=3600`
- `FORCE_CACHE_ALL` applies its default TTL to every response, manifests included. For a live or
  rolling stream keep it short, or a cached `.m3u8`/`.mpd` freezes playback.
- The TTL never extends token validity, because the plugin checks the token before the cache lookup.

## Runtime notes

- **CPU and time**: a plugin gets 1 ms of CPU per request; a plugin that exceeds the limit is
  terminated and the request answers `500`. Keep the plugin on the `REQUEST_HEADERS` event only.
- **Size**: the plugin plus its configuration must stay under 5 MiB. `packaging/build.sh` optimizes
  the module with `wasm-opt` and fails if the module plus `config.json` is too large.
- **Failure mode**: with `failOpen: false` (the setting below) a plugin failure answers `500`
  instead of letting an unwatermarked request through.
- **No outbound calls** are available or needed; the configuration is read once at start-up.
- **Logs**: enable them with `--log-config` when you create the plugin. They go to Cloud Logging under
  `resource.type="networkservices.googleapis.com/WasmPluginVersion"`. At `INFO` the plugin logs every
  request path (token included), the decoded payload (the symbol pattern on the Akamai WMT route) and
  the resolved path or the rejection reason. Once the integration works, lower `sample-rate` or set
  `min-log-level=ERROR`.

## Deployment

### Prerequisites
- A **Google Cloud project** with the Network Services and Artifact Registry APIs enabled
  (`gcloud services enable networkservices.googleapis.com artifactregistry.googleapis.com`)
- A **global external Application Load Balancer** (scheme `EXTERNAL_MANAGED`; the classic Application
  Load Balancer cannot run extensions) whose backend bucket has **Cloud CDN** enabled and points at a
  **private** Cloud Storage bucket holding the watermarked renditions. Let the load balancer read the
  bucket; its service agent appears only after the project's first backend bucket is created:

  ```bash
  gcloud storage buckets add-iam-policy-binding gs://BUCKET_NAME \
      --member=serviceAccount:service-PROJECT_NUMBER@https-lb.iam.gserviceaccount.com \
      --role=roles/storage.objectViewer
  ```
- An **Artifact Registry** Docker repository in the multi-regional `us` location
- The **gcloud CLI** and **Docker**
- To build: **Go 1.24+** and **binaryen** (`wasm-opt`). To deploy a prebuilt module you need neither
  Go nor binaryen.

### Step 1: Build or download the plugin

```bash
./packaging/build.sh
```

This writes `dist/plugin.wasm`. Each GitHub Release also attaches
`doverunner_fwm_embedder_gcp.<version>.zip`, holding a prebuilt `plugin.wasm` with `config.json`,
`Dockerfile` and `LICENSE`. If you use it, run Steps 2 and 3 from the unzipped folder.

### Step 2: Push the plugin to Artifact Registry

`packaging/Dockerfile` packages the module as an OCI image (`FROM scratch`, `COPY plugin.wasm plugin.wasm`).
If the repository does not exist yet, create it once with
`gcloud artifacts repositories create REPOSITORY --repository-format=docker --location=us`.

```bash
gcloud auth configure-docker us-docker.pkg.dev
docker build --no-cache --platform wasm -f packaging/Dockerfile -t fwm-embedder dist/
docker tag fwm-embedder us-docker.pkg.dev/PROJECT_ID/REPOSITORY/fwm-embedder:3.0.0
docker push us-docker.pkg.dev/PROJECT_ID/REPOSITORY/fwm-embedder:3.0.0
```

From the unzipped release, build with `docker build --no-cache --platform wasm -t fwm-embedder .` instead.

### Step 3: Create the plugin with your configuration

```bash
gcloud service-extensions wasm-plugins create fwm-embedder \
    --location=global \
    --image=us-docker.pkg.dev/PROJECT_ID/REPOSITORY/fwm-embedder:3.0.0 \
    --main-version=v3-0-0 \
    --plugin-config-file=config.json \
    --log-config=enable=true,sample-rate=1,min-log-level=INFO
```

### Step 4: Attach it to the load balancer as an edge extension

Save this as `edge-extension.yaml`. The match condition decides which requests reach the plugin: write
one `request.path.startsWith("/<folder>/")` term for **each** `prefixFolder` entry in `config.json`, and
keep `request.path.startsWith("/wmt:")` if you accept Akamai WMT tokens. A request the condition does not
match is served from the bucket as is. The whole `celExpression` may hold at most 500 characters.

```yaml
name: fwm-embedder
forwardingRules:
- https://www.googleapis.com/compute/v1/projects/PROJECT_ID/global/forwardingRules/HTTPS_FORWARDING_RULE
- https://www.googleapis.com/compute/v1/projects/PROJECT_ID/global/forwardingRules/HTTP_FORWARDING_RULE
loadBalancingScheme: EXTERNAL_MANAGED
extensionChains:
- name: watermark
  matchCondition:
    celExpression: 'request.path.startsWith("/wm-contents/") || request.path.startsWith("/wmt:")'
  extensions:
  - name: fwm-embedder
    service: projects/PROJECT_ID/locations/global/wasmPlugins/fwm-embedder
    failOpen: false
    supportedEvents:
    - REQUEST_HEADERS
```

List **every** forwarding rule whose URL map reaches the bucket: usually the HTTPS rule, plus the HTTP
rule unless it only redirects to HTTPS. A request on an unlisted rule skips the plugin, and a forwarding
rule can carry only one edge extension.
`gcloud compute forwarding-rules describe FORWARDING_RULE --global --format='value(selfLink)'` prints a
rule's reference.

```bash
gcloud service-extensions lb-edge-extensions import fwm-embedder \
    --source=edge-extension.yaml \
    --location=global
```

The plugin takes a few minutes to reach every location. Then request a tokenized URL through the
load balancer and check that the second viewer of the same rendition is served from cache: the response
carries an `Age` header, and the load balancer request log (`resource.type="http_load_balancer"`) shows
`httpRequest.cacheHit: true`.

### Step 5: Allow browser players (CORS)

A player served from another origin needs CORS headers on manifests and segments. Add them to the
backend bucket, replacing `*` with your player's origin where you can:

```bash
gcloud compute backend-buckets update BACKEND_BUCKET \
    --custom-response-header='Access-Control-Allow-Origin: *' \
    --custom-response-header='Access-Control-Allow-Methods: GET, HEAD, OPTIONS' \
    --custom-response-header='Access-Control-Allow-Headers: Range' \
    --custom-response-header='Access-Control-Expose-Headers: Content-Length, Content-Range'
```

### Updating the plugin or its configuration

To ship a new build or a changed `config.json`, create a new plugin version: pass `--image` (the same
tag is fine when only the configuration changed), a new `--main-version` and the configuration.
`--plugin-config-file` is accepted only together with `--image`:

```bash
gcloud service-extensions wasm-plugins update fwm-embedder \
    --location=global \
    --image=us-docker.pkg.dev/PROJECT_ID/REPOSITORY/fwm-embedder:3.0.1 \
    --main-version=v3-0-1 \
    --plugin-config-file=config.json
```

To roll back, switch to an existing version without `--image`. A plugin can hold at most 100 versions;
delete one that is not the main version with
`gcloud service-extensions wasm-plugin-versions delete VERSION --wasm-plugin=fwm-embedder --location=global`.

```bash
gcloud service-extensions wasm-plugins update fwm-embedder \
    --location=global \
    --main-version=v3-0-0
```

### Verifying the sample locally

Before you push an image, check that the sources are formatted and the module fits:

```bash
gofmt -l .
go vet ./...
./packaging/build.sh
```

`gofmt -l` prints nothing when the sources are formatted. `packaging/build.sh` prints the module size
and fails when the module plus `config.json` exceeds 5 MiB. The plugin itself runs only inside the load
balancer: after Step 4, request a tokenized URL and read the plugin log.

### Media CDN

This sample is not tested on Media CDN. Google does not recommend the Go SDK there: a Media CDN plugin
gets 4 MiB of memory per instance, and the Go runtime alone needs 2–3 MiB. If you try it anyway, create a
separate `wasmPlugins` resource for the `CdnEdgeExtension` (Preview) — a plugin used by a Media CDN
extension cannot also serve a Cloud Load Balancing extension. See
[Media CDN extensions](https://docs.cloud.google.com/service-extensions/docs/media-cdn-extensions-overview).

### Additional Resources

- [Service Extensions plugins overview](https://docs.cloud.google.com/service-extensions/docs/plugins-overview)
- [Prepare the plugin code](https://docs.cloud.google.com/service-extensions/docs/prepare-plugin-code)
- [Create a plugin](https://docs.cloud.google.com/service-extensions/docs/create-plugin)
- [Configure edge extensions](https://docs.cloud.google.com/service-extensions/docs/configure-edge-extensions)
- [Set up a backend bucket](https://docs.cloud.google.com/cdn/docs/setting-up-cdn-with-bucket)
- [Quotas and limits](https://docs.cloud.google.com/service-extensions/docs/quotas)
