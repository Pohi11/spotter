import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";

import { Arrow, Box, Diagram, Note, Region, Store } from "@/components/diagram";
import { Code, Dot, PageTitle, Points, Section, link } from "@/components/ui";
import { loadResults } from "@/lib/results";

// The architecture page (spec: dashboard "Architecture diagrams have text equivalents").
// The diagrams are drawn by hand and name the modules they show, so a reader can find each
// part in the code. Decision records are linked where a choice is explained.

export const metadata: Metadata = { title: "Architecture" };

const REPO = "https://github.com/Pohi11/spotter/blob/main/";

function Decision({ n, file, children }: { n: string; file: string; children?: ReactNode }) {
  return (
    <a href={`${REPO}docs/decisions/${n}-${file}.md`} className={link} target="_blank" rel="noopener noreferrer">
      {children ?? `decision ${n}`}
    </a>
  );
}

export default function ArchitecturePage() {
  return (
    <>
      <PageTitle title="Architecture">
        <p>
          Spotter is three Go services, a Python worker, and a Python trainer around NATS
          JetStream, PostgreSQL, and an S3-compatible object store. The services never call each
          other to hand off work: a job or a state change is an event on the bus, defined in
          protobuf, delivered at least once, and handled idempotently.
        </p>
      </PageTitle>
      <Legend />
      <Parts />
      <TilePath />
      <Lifecycle />
      <Guarantees />
      <Hexagon />
      <Kubernetes />
      <Tracing />
      <Subjects />
    </>
  );
}

function Legend() {
  const items = [
    { tone: "code", label: "Go service" },
    { tone: "model", label: "Python and the model" },
    { tone: "bus", label: "Event bus" },
    { tone: "human", label: "A person decides" },
    { tone: "plain", label: "TypeScript dashboard or a client" },
  ] as const;
  return (
    <div className="mt-8 flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted">
      {items.map((k) => (
        <span key={k.label} className="inline-flex items-center gap-2">
          <Dot tone={k.tone} />
          {k.label}
        </span>
      ))}
      <span className="inline-flex items-center gap-2">
        <span aria-hidden className="inline-block h-3 w-5 rounded-sm border border-dashed border-faint" />
        Outside the platform, or runs on demand
      </span>
    </div>
  );
}

// --- Parts ------------------------------------------------------------------------------

function Parts() {
  const id = "parts";
  return (
    <Section
      id={id}
      title="Parts and connections"
      intro="Each arrow points from the part that makes a call to the part it calls. Everything to the right of the gateway is internal; the gateway is the only public entry point."
    >
      <Diagram
        id={`${id}-diagram`}
        className="mt-10"
        title="System parts"
        width={1020}
        height={545}
        minWidth={820}
        description={
          <p>
            Browsers load the Next.js dashboard, which reaches the gateway through an allowlisted
            proxy. Other clients call the gateway&apos;s REST or gRPC APIs directly. The gateway calls
            the registry over gRPC, stores tiles in the object store, and publishes jobs and
            reads events on NATS JetStream. The registry stores versions in PostgreSQL and
            publishes its outbox to JetStream. The inference worker pulls jobs from JetStream,
            looks up versions in the registry, reads tiles and model artifacts from the object
            store, and publishes results. The drift monitor consumes completions from JetStream,
            reads each version&apos;s reference from the object store, and publishes drift events. The
            trainer writes artifacts to the object store and registers versions.
          </p>
        }
      >
        <Box x={20} y={50} w={150} h={80} tone="plain" dashed title="Browser" sub={["dashboard pages"]} />
        <Box x={200} y={50} w={170} h={80} tone="plain" title="Dashboard" sub={["Next.js,", "proxy at /api"]} />
        <Box x={20} y={250} w={150} h={80} tone="plain" dashed title="Other clients" sub={["curl, grpcurl,", "spotterload"]} />
        <Box x={410} y={150} w={170} h={150} tone="code" title="Gateway" sub={["REST and gRPC,", "event feed,", "job submission"]} />
        <Box x={650} y={30} w={170} h={90} tone="code" title="Registry" sub={["versions, gates,", "outbox relay"]} />
        <Store x={860} y={30} w={140} h={90} title="PostgreSQL" sub="registry data" />
        <Box x={620} y={200} w={200} h={110} tone="bus" title="NATS JetStream" sub={["INFERENCE, REGISTRY,", "MONITOR streams,", "jobs key-value bucket"]} />
        <Box x={845} y={205} w={165} h={100} tone="model" title="Inference worker" sub={["ONNX Runtime,", "production + shadow"]} />
        <Store x={410} y={400} w={170} h={100} title="Object store" sub="SeaweedFS, S3 API" />
        <Box x={650} y={400} w={170} h={100} tone="code" title="Drift monitor" sub={["PSI and KS", "per version"]} />
        <Box x={860} y={400} w={140} h={100} tone="model" dashed title="Trainer" sub={["PyTorch,", "Kubernetes Job"]} />

        <Arrow diagram={`${id}-diagram`} d="M170 90 H200" />
        <Arrow diagram={`${id}-diagram`} d="M370 90 C395 90 390 180 410 180" label="REST, SSE" lx={392} ly={140} anchor="start" />
        <Arrow diagram={`${id}-diagram`} d="M170 290 H410" label="REST, gRPC" lx={290} ly={282} />
        <Arrow diagram={`${id}-diagram`} d="M580 175 C615 175 615 75 650 75" label="gRPC" lx={622} ly={118} />
        <Arrow diagram={`${id}-diagram`} d="M820 75 H860" />
        <Arrow diagram={`${id}-diagram`} d="M735 120 V200" ink="bus" label="outbox relay" lx={745} ly={168} anchor="start" />
        <Arrow diagram={`${id}-diagram`} d="M580 255 H620" ink="bus" />
        <Arrow diagram={`${id}-diagram`} d="M495 300 V400" label="tiles" lx={505} ly={355} anchor="start" />
        <Arrow diagram={`${id}-diagram`} d="M845 255 H820" ink="bus" />
        <Arrow diagram={`${id}-diagram`} d="M885 205 C885 165 845 125 805 120" label="versions" lx={893} ly={172} anchor="start" />
        <Arrow diagram={`${id}-diagram`} d="M900 305 V350 H560 V400" label="tiles, models" lx={790} ly={344} />
        <Arrow diagram={`${id}-diagram`} d="M735 400 V310" ink="bus" label="completions" lx={745} ly={388} anchor="start" />
        <Arrow diagram={`${id}-diagram`} d="M650 455 H580" label="reference" lx={615} ly={446} />
        <Arrow diagram={`${id}-diagram`} d="M930 500 V525 H495 V500" label="artifacts" lx={700} ly={520} />
      </Diagram>

      <div className="mt-10 grid gap-x-14 gap-y-4 lg:grid-cols-2">
        <Points
          items={[
            <>
              <span className="text-ink">Gateway</span> (<Code>services/gateway</Code>): the public front
              door. One protobuf file defines its gRPC services, the REST paths (grpc-gateway), and
              the OpenAPI document (<Decision n="0007" file="grpc-go-with-grpc-gateway" />). It also
              serves <Code>/v1/events</Code>, the live feed this dashboard reads (<Decision n="0016" file="dashboard-and-gateway-event-feed" />).
            </>,
            <>
              <span className="text-ink">Registry</span> (<Code>services/registry</Code>): model versions,
              the stage machine, the promotion gates, approvals, and shadow statistics in Postgres.
              Every change writes its event to an outbox table in the same transaction.
            </>,
            <>
              <span className="text-ink">Drift monitor</span> (<Code>services/monitor</Code>): a window
              per version compared with that version&apos;s training-time reference (<Decision n="0012" file="drift-detection-psi-against-training-reference" />).
            </>,
          ]}
        />
        <Points
          items={[
            <>
              <span className="text-ink">Inference worker</span> (<Code>workers/inference</Code>): pulls
              jobs, verifies each artifact&apos;s SHA-256 before loading it, serves the production
              version, and runs the newest shadow version beside it.
            </>,
            <>
              <span className="text-ink">Trainer</span> (<Code>workers/trainer</Code>): trains, evaluates on
              the holdout split, exports and quantizes, writes content-addressed artifacts (<Decision n="0008" file="content-addressed-model-artifacts" />),
              and registers each version with its metrics.
            </>,
            <>
              <span className="text-ink">No cloud SDK outside an adapter.</span> The object store is
              reached through an S3 port and tested against SeaweedFS (<Decision n="0003" file="object-store-seaweedfs" />), so the
              platform runs the same on a laptop, in k3d, or in any cluster.
            </>,
          ]}
        />
      </div>
    </Section>
  );
}

// --- A tile's path ------------------------------------------------------------------------

const LANES = [
  { x: 80, label: "Client", tone: "plain" as const },
  { x: 250, label: "Gateway", tone: "code" as const },
  { x: 420, label: "Object store", tone: "plain" as const },
  { x: 590, label: "JetStream", tone: "bus" as const },
  { x: 760, label: "Worker", tone: "model" as const },
  { x: 920, label: "Monitor", tone: "code" as const },
];

function Msg({ id, from, to, y, label, dashed, ink = "ink" }: { id: string; from: number; to: number; y: number; label: string; dashed?: boolean; ink?: "ink" | "bus" | "model" }) {
  const a = LANES[from].x;
  const b = LANES[to].x;
  const d = `M${a + (b > a ? 4 : -4)} ${y} H${b + (b > a ? -6 : 6)}`;
  return <Arrow diagram={id} d={d} dashed={dashed} ink={ink} label={label} lx={(a + b) / 2} ly={y - 7} />;
}

function TilePath() {
  const id = "path-diagram";
  return (
    <Section
      id="path"
      title="A tile's path"
      intro="Submitting is asynchronous: the client gets a job id as soon as the job is safely on the bus, and the result is written once, whatever crashes in between."
    >
      <Diagram
        id={id}
        className="mt-10"
        title="One classification, in order"
        width={1000}
        height={600}
        minWidth={820}
        description={
          <ol>
            <li>The client sends a tile to the gateway.</li>
            <li>The gateway puts the tile in the object store.</li>
            <li>It records the job as pending in the JetStream key-value bucket.</li>
            <li>It publishes inference.requested and returns the job id.</li>
            <li>JetStream delivers the job to one worker, which reads the tile and classifies it.</li>
            <li>The worker sets the job to succeeded with a compare-and-set, publishes inference.completed, and then acknowledges.</li>
            <li>The monitor consumes the completion and adds it to the version&apos;s drift window.</li>
            <li>The client reads the result by job id.</li>
          </ol>
        }
      >
        {LANES.map((l) => (
          <g key={l.label}>
            <line x1={l.x} y1={68} x2={l.x} y2={585} className="stroke-rule" strokeWidth={1.5} strokeDasharray="4 4" />
            <Box x={l.x - 70} y={14} w={140} h={50} tone={l.tone} title={l.label} />
          </g>
        ))}
        <Msg id={id} from={0} to={1} y={110} label="SubmitInference(tile)" />
        <Msg id={id} from={1} to={2} y={150} label="put tile" />
        <Msg id={id} from={1} to={3} y={190} label="job = PENDING (key-value)" ink="bus" />
        <Msg id={id} from={1} to={3} y={230} label="publish inference.requested" ink="bus" />
        <Msg id={id} from={1} to={0} y={270} label="job id" dashed />
        <Msg id={id} from={3} to={4} y={320} label="deliver (pull consumer)" ink="bus" />
        <Msg id={id} from={4} to={2} y={360} label="get tile" />
        <rect x={730} y={378} width={60} height={30} rx={6} className="fill-model-soft stroke-model" strokeWidth={1.5} />
        <Note x={722} y={398} anchor="end" muted={false}>classify (production, then shadow)</Note>
        <Msg id={id} from={4} to={3} y={440} label="job = SUCCEEDED (compare-and-set)" ink="bus" />
        <Msg id={id} from={4} to={3} y={480} label="publish inference.completed" ink="bus" />
        <Msg id={id} from={4} to={3} y={520} label="ack" ink="bus" />
        <Msg id={id} from={3} to={5} y={545} label="completion" ink="bus" />
        <Msg id={id} from={0} to={1} y={575} label="GetInference(job id)" />
      </Diagram>
      <Points
        className="mt-8 max-w-3xl"
        items={[
          <>
            Job state lives in a JetStream key-value bucket, not Postgres: it is short-lived,
            keyed by id, and needs compare-and-set, which the bucket gives per key (<Decision n="0009" file="at-least-once-inference-with-kv-job-state" />).
          </>,
          <>
            Every publish carries a <Code>Nats-Msg-Id</Code> (<Code>requested-&lt;job&gt;</Code>,{" "}
            <Code>completed-&lt;job&gt;</Code>), so JetStream drops a duplicate published within its
            two-minute window.
          </>,
          <>
            A job that fails on every delivery, or can&apos;t be decoded, is marked failed and
            published on <Code>inference.failed.&lt;model&gt;</Code>, the dead-letter subject.
          </>,
        ]}
      />
    </Section>
  );
}

// --- Lifecycle ----------------------------------------------------------------------------

function Lifecycle() {
  const id = "lifecycle-diagram";
  const S = { y: 110, h: 64, w: 130 };
  return (
    <Section
      id="lifecycle"
      title="A version's lifecycle"
      intro={
        <>
          A version&apos;s stage is a state machine in code (<Code>services/registry/internal/domain/lifecycle.go</Code>) and
          in the database. Every move not drawn here is refused, and every move is recorded with its reason.
        </>
      }
    >
      <Diagram
        id={id}
        className="mt-10"
        title="Stages and the moves between them"
        width={1020}
        height={330}
        minWidth={820}
        description={
          <p>
            A version is registered, then validated if its holdout accuracy meets the floor, or
            rejected. A validated version can go to shadow or be rejected. A shadow version goes
            to production only when every gate passes and a person has approved it, or it is
            rejected. A production version is retired when another replaces it, or rolled back.
            A rollback restores the most recently retired version to production.
          </p>
        }
      >
        <Box x={20} y={S.y} w={S.w} h={S.h} tone="code" title="registered" />
        <Box x={235} y={S.y} w={S.w} h={S.h} tone="code" title="validated" />
        <Box x={450} y={S.y} w={S.w} h={S.h} tone="wait" title="shadow" />
        <Box x={665} y={S.y} w={S.w} h={S.h} tone="pass" title="production" />
        <Box x={880} y={S.y} w={S.w} h={S.h} tone="plain" title="retired" />
        <Box x={235} y={250} w={S.w} h={56} tone="fail" title="rejected" />
        <Box x={665} y={250} w={S.w} h={56} tone="fail" title="rolled back" />

        <Arrow diagram={id} d="M150 142 H235" />
        <Note x={192} y={98} anchor="middle">accuracy ≥ floor</Note>
        <Arrow diagram={id} d="M365 142 H450" />
        <Arrow diagram={id} d="M580 142 H665" ink="human" heavy />
        <Note x={622} y={98} anchor="middle">gates + approval</Note>
        <Arrow diagram={id} d="M795 142 H880" />
        <Note x={837} y={98} anchor="middle">replaced</Note>
        <Arrow diagram={id} d="M945 110 C945 40 740 40 740 106" ink="fail" label="restored by rollback" lx={842} ly={44} />
        <Arrow diagram={id} d="M730 174 V250" ink="fail" label="rollback" lx={740} ly={218} anchor="start" />
        <Arrow diagram={id} d="M85 174 C85 278 160 278 235 278" ink="faint" />
        <Arrow diagram={id} d="M300 174 V250" ink="faint" />
        <Arrow diagram={id} d="M515 174 C515 278 440 278 365 278" ink="faint" />
      </Diagram>
      <Points
        className="mt-8 max-w-3xl"
        items={[
          <>
            Postgres holds the invariant too: a partial unique index allows one production
            version per model, and every update names the revision it expects, so of two racing
            transitions exactly one wins.
          </>,
          <>
            Promotion and rollback each swap two versions in one transaction, so there is never a
            moment with no production version or two (<Decision n="0013" file="gated-promotion-shadow-traffic-human-approval" />).
          </>,
          <>
            The gates are plain Go: holdout accuracy within 0.01 of production, at least 200 shadow
            comparisons agreeing 95% of the time, mean shadow latency within 25 ms, and a
            recorded approval. A model&apos;s first version has nothing to compare with, so the
            approval alone decides (<Decision n="0015" file="first-promotion-without-shadow-evidence" />).
          </>,
        ]}
      />
    </Section>
  );
}

// --- Delivery guarantees ------------------------------------------------------------------

function Guarantees() {
  const id = "delivery-diagram";
  return (
    <Section
      id="delivery"
      title="Delivery guarantees"
      intro="JetStream delivers at least once. Two patterns turn that into exactly one effect: the registry never loses an event, and the worker never records a job twice."
    >
      <Diagram
        id={id}
        className="mt-10"
        title="The registry's outbox and the worker's idempotent processing"
        width={1000}
        height={420}
        minWidth={820}
        description={
          <>
            <p>
              Registry: one Postgres transaction updates the version and inserts an outbox row.
              The relay reads unpublished rows in order, publishes each with its event id as the
              message id, and marks it published only after JetStream acknowledges.
            </p>
            <p>
              Worker: it fetches a job, returns early if the job is no longer pending, classifies
              the tile, sets the job to succeeded only if it is still pending, publishes the
              completion with a de-duplicating message id, and acknowledges last. A crash before
              the acknowledgement means a redelivery that finds the job done.
            </p>
          </>
        }
      >
        <Region x={20} y={20} w={460} h={380} label="Registry (Go)" />
        <rect x={45} y={60} width={410} height={130} rx={12} className="fill-surface stroke-code" strokeWidth={1.5} strokeDasharray="6 4" />
        <Note x={60} y={84} muted={false} bold>One Postgres transaction</Note>
        <Box x={60} y={100} w={180} h={70} tone="code" title="UPDATE version" sub={["stage, revision + 1"]} />
        <Box x={260} y={100} w={180} h={70} tone="code" title="INSERT outbox row" sub={["protobuf event"]} />
        <Box x={60} y={235} w={180} h={70} tone="code" title="Outbox relay" sub={["reads rows in order"]} />
        <Box x={260} y={235} w={180} h={70} tone="bus" title="Publish" sub={["Nats-Msg-Id = event id"]} />
        <Box x={160} y={320} w={180} h={60} tone="code" title="Mark published" sub={["after the server's ack"]} />
        <Arrow diagram={id} d="M350 170 C350 205 150 200 150 235" label="committed" lx={250} ly={205} />
        <Arrow diagram={id} d="M240 270 H260" />
        <Arrow diagram={id} d="M350 305 C350 330 340 340 340 345" />

        <Region x={520} y={20} w={460} h={380} label="Inference worker (Python)" />
        <Box x={545} y={55} w={190} h={60} tone="bus" title="Fetch job" sub={["durable pull consumer"]} />
        <Box x={765} y={55} w={190} h={60} tone="model" title="Still pending?" sub={["no: ack and stop"]} />
        <Box x={765} y={140} w={190} h={60} tone="model" title="Classify" />
        <Box x={765} y={225} w={190} h={60} tone="bus" title="Compare-and-set" sub={["PENDING to SUCCEEDED"]} />
        <Box x={545} y={225} w={190} h={60} tone="bus" title="Publish completion" sub={["completed-<job id>"]} />
        <Box x={545} y={315} w={190} h={60} tone="bus" title="Ack" sub={["only now"]} />
        <Arrow diagram={id} d="M735 85 H765" />
        <Arrow diagram={id} d="M860 115 V140" />
        <Arrow diagram={id} d="M860 200 V225" />
        <Arrow diagram={id} d="M765 255 H735" />
        <Arrow diagram={id} d="M640 285 V315" />
        <Arrow diagram={id} d="M545 345 C510 345 510 85 545 85" ink="fail" dashed label="crash: redelivered" lx={528} ly={210} anchor="middle" />
      </Diagram>
      <Points
        className="mt-8 max-w-3xl"
        items={[
          "Without the outbox, a crash between the database commit and the publish would lose the event, or a publish before a rollback would announce a change that never happened.",
          "Tests stop the worker after classifying and before recording, and after recording and before acknowledging; each job still ends with one result and one completion event.",
          "The registry's shadow-statistics consumer is idempotent too: comparisons are keyed by job id, so a redelivered comparison is counted once.",
        ]}
      />
    </Section>
  );
}

// --- Hexagonal ----------------------------------------------------------------------------

function Hexagon() {
  const id = "hexagon-diagram";
  return (
    <Section
      id="hexagonal"
      title="Inside a Go service"
      intro="Each Go service has the same layout. The rules and use cases know nothing about Postgres, NATS, S3, gRPC, or HTTP; adapters plug into ports from the outside."
    >
      <Diagram
        id={id}
        className="mt-10"
        title="Ports and adapters in the registry"
        width={1000}
        height={360}
        minWidth={760}
        description={
          <p>
            The domain package holds pure types and rules. The app package holds use cases and
            depends only on the domain and on port interfaces. Adapters implement the ports
            (Postgres, an in-memory store, the NATS publisher) or drive the use cases (the gRPC
            server). An architecture test fails the build if the domain, ports, or app packages
            import infrastructure.
          </p>
        }
      >
        <rect x={180} y={20} width={640} height={320} rx={28} className="fill-wash stroke-faint" strokeWidth={1.5} />
        <Note x={200} y={46} muted={false} bold>adapters</Note>
        <rect x={250} y={60} width={500} height={240} rx={24} className="fill-surface stroke-code" strokeWidth={1.5} />
        <Note x={270} y={86} muted={false} bold>ports (interfaces)</Note>
        <rect x={320} y={100} width={360} height={160} rx={20} className="fill-code-soft stroke-code" strokeWidth={1.5} />
        <Note x={340} y={126} muted={false} bold>app (use cases)</Note>
        <rect x={390} y={140} width={220} height={90} rx={16} className="fill-surface stroke-ink" strokeWidth={1.5} />
        <text x={500} y={178} textAnchor="middle" className="fill-ink text-[15px] font-semibold">domain</text>
        <text x={500} y={198} textAnchor="middle" className="fill-muted text-[13px]">stages, transitions,</text>
        <text x={500} y={214} textAnchor="middle" className="fill-muted text-[13px]">gates, errors</text>

        <Box x={10} y={80} w={160} h={60} tone="code" title="gRPC server" sub={["drives use cases"]} />
        <Box x={10} y={200} w={160} h={60} tone="bus" title="Shadow consumer" sub={["drives use cases"]} />
        <Box x={830} y={60} w={160} h={60} tone="code" title="Postgres repo" sub={["implements a port"]} />
        <Box x={830} y={150} w={160} h={60} tone="plain" title="In-memory repo" sub={["for unit tests"]} />
        <Box x={830} y={240} w={160} h={60} tone="bus" title="NATS publisher" sub={["outbox relay"]} />
        <Arrow diagram={id} d="M170 110 H320" />
        <Arrow diagram={id} d="M170 230 H320" />
        <Arrow diagram={id} d="M830 90 H750" />
        <Arrow diagram={id} d="M830 180 H750" />
        <Arrow diagram={id} d="M830 270 H750" />
      </Diagram>
      <Points
        className="mt-8 max-w-3xl"
        items={[
          <>
            <Code>archtest</Code> runs <Code>go list -deps</Code> on the core packages and fails if
            any imports <Code>net</Code>, <Code>database</Code>, or a third-party module other than
            the UUID library. Adding a real <Code>net/http</Code> import to the domain made it fail,
            naming the import.
          </>,
          "The Postgres and in-memory repositories pass the same contract suite, including ten concurrent registrations numbered exactly 1 to 10 and two racing transitions with exactly one winner.",
          "Domain errors map to gRPC status codes in one function, and grpc-gateway maps those to HTTP statuses, so an illegal move is FAILED_PRECONDITION over gRPC and 400 over REST, from every caller.",
        ]}
      />
    </Section>
  );
}

// --- Kubernetes ---------------------------------------------------------------------------

function Kubernetes() {
  const id = "k8s-diagram";
  const { spike } = loadResults();
  return (
    <Section
      id="kubernetes"
      title="On Kubernetes"
      intro="One Helm umbrella chart installs the whole platform into an empty namespace. CI installs it into a kind cluster on every pull request and every push to main, and runs a smoke test against it."
    >
      <Diagram
        id={id}
        className="mt-10"
        title="What the chart creates"
        width={1000}
        height={400}
        minWidth={820}
        description={
          <p>
            The chart creates Deployments for the dashboard, gateway, registry, drift monitor, and
            inference worker; StatefulSets for PostgreSQL, SeaweedFS, and NATS from the official
            NATS chart; a generated Secret with the database and object-store credentials; and
            an optional trainer Job. KEDA reads the inference consumer&apos;s lag from NATS and scales
            the worker Deployment between its minimum and maximum replicas.
          </p>
        }
      >
        <Region x={20} y={20} w={760} h={360} label="Helm release spotter (deploy/helm/spotter)" />
        <Note x={45} y={70}>Deployments</Note>
        <Box x={45} y={82} w={130} h={56} tone="plain" title="dashboard" />
        <Box x={190} y={82} w={130} h={56} tone="code" title="gateway" />
        <Box x={335} y={82} w={130} h={56} tone="code" title="registry" />
        <Box x={480} y={82} w={130} h={56} tone="code" title="monitor" />
        <Box x={625} y={82} w={130} h={56} tone="model" title="worker" sub={["1 to N replicas"]} />
        <Note x={45} y={180}>StatefulSets</Note>
        <Box x={45} y={192} w={170} h={60} tone="store" title="postgres" sub={["or an external DB"]} />
        <Box x={235} y={192} w={170} h={60} tone="store" title="seaweedfs" sub={["S3 API"]} />
        <Box x={425} y={192} w={170} h={60} tone="bus" title="nats" sub={["official chart, vendored"]} />
        <Note x={45} y={292}>Configuration and jobs</Note>
        <Box x={45} y={304} w={170} h={56} tone="plain" title="Secret" sub={["generated once"]} />
        <Box x={235} y={304} w={170} h={56} tone="model" dashed title="trainer Job" />
        <Box x={425} y={304} w={170} h={56} tone="plain" dashed title="helm test" sub={["spotterctl smoke"]} />
        <Box x={820} y={150} w={160} h={90} tone="plain" title="KEDA" sub={["ScaledObject on", "consumer lag"]} />
        <Arrow diagram={id} d="M820 200 C700 210 660 222 595 222" ink="bus" label="reads lag" lx={720} ly={200} />
        <Arrow diagram={id} d="M880 150 C880 110 820 110 755 110" label="scales" lx={830} ly={104} />
      </Diagram>
      <Points
        className="mt-8 max-w-3xl"
        items={[
          <>
            Workers scale on queue depth, not CPU: a job waiting on the stream is the signal that
            matters (<Decision n="0014" file="helm-umbrella-chart-keda-scaling" />). On a local k3d cluster
            a {spike.autoscaled.concurrency}-client spike ran at {spike.autoscaled.report.completed_per_second.toFixed(1)} jobs/s with KEDA
            against {spike.oneWorker.report.completed_per_second.toFixed(1)} with one worker.
          </>,
          "Every container has probes, requests, and limits; credentials reach workloads only through Secret references; each service drains in-flight work on SIGTERM within its grace period.",
          <>
            Locally, <Code>tilt up</Code> builds the images and installs KEDA and the chart into k3d;
            from an empty cluster to ready took 7 minutes 29 seconds.
          </>,
        ]}
      />
    </Section>
  );
}

// --- Tracing ------------------------------------------------------------------------------

function Tracing() {
  return (
    <Section
      id="tracing"
      title="One trace per request"
      intro="Every request is one OpenTelemetry trace, across HTTP, gRPC, and the bus (decision 0010)."
    >
      <div className="mt-8 grid gap-x-14 gap-y-4 lg:grid-cols-2">
        <Points
          items={[
            "W3C trace context travels in HTTP and gRPC headers through the standard instrumentation, and in NATS message headers through a small shared package (internal/natstrace).",
            "The trace even survives the outbox: the registry stores the active traceparent with each outbox row, and the relay attaches it when publishing, so an operator's promotion and the worker's reload land in one trace.",
          ]}
        />
        <Points
          items={[
            "Spans carry ids, versions, outcomes, and sizes, never image bytes or credentials; a test records every span of a submission and checks.",
            <>
              Every service serves Prometheus metrics at <Code>/metrics</Code>: request rates and
              durations, jobs by outcome, model time, consumer lag, and PSI per feature. The{" "}
              <Link href="/#example" className={link}>worked example</Link> on the home page is one
              of these traces.
            </>,
          ]}
        />
      </div>
    </Section>
  );
}

// --- Subjects -----------------------------------------------------------------------------

const SUBJECTS: { subject: string; message: string; from: string; to: string }[] = [
  { subject: "inference.requested.<model>", message: "InferenceRequested", from: "gateway", to: "inference worker" },
  { subject: "inference.completed.<model>", message: "InferenceCompleted", from: "inference worker", to: "drift monitor, event feed" },
  { subject: "inference.shadow.<model>", message: "ShadowCompared", from: "inference worker", to: "registry (shadow statistics)" },
  { subject: "inference.failed.<model>", message: "InferenceFailed", from: "inference worker", to: "event feed (dead letters)" },
  { subject: "registry.events.model_version_registered", message: "ModelVersionRegistered", from: "registry outbox", to: "event feed" },
  { subject: "registry.events.model_version_stage_changed", message: "ModelVersionStageChanged", from: "registry outbox", to: "inference worker (reload), event feed" },
  { subject: "monitor.drift.evaluated.<model>", message: "DriftEvaluated", from: "drift monitor", to: "event feed (drift charts)" },
  { subject: "monitor.drift.detected.<model>", message: "DriftDetected", from: "drift monitor", to: "event feed" },
  { subject: "monitor.drift.resolved.<model>", message: "DriftResolved", from: "drift monitor", to: "event feed" },
];

function Subjects() {
  return (
    <Section
      id="subjects"
      title="Events on the bus"
      intro="Every event has a protobuf schema in proto/spotter/v1, and buf breaking guards it in CI. Streams and the bucket are defined in code (internal/natsjs), never by hand."
    >
      <div className="mt-8 overflow-x-auto">
        <table className="w-full min-w-[44rem] border-t border-ink text-left text-sm">
          <thead>
            <tr className="text-xs text-faint">
              <th scope="col" className="py-3 pr-4 font-medium">Subject</th>
              <th scope="col" className="py-3 pr-4 font-medium">Message</th>
              <th scope="col" className="py-3 pr-4 font-medium">Published by</th>
              <th scope="col" className="py-3 font-medium">Read by</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule border-t border-rule">
            {SUBJECTS.map((s) => (
              <tr key={s.subject}>
                <td className="py-3 pr-4 font-mono text-xs">{s.subject}</td>
                <td className="py-3 pr-4">{s.message}</td>
                <td className="py-3 pr-4 text-muted">{s.from}</td>
                <td className="py-3 text-muted">{s.to}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  );
}
