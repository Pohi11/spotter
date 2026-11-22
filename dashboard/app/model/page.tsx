import type { Metadata } from "next";
import Link from "next/link";

import { Promotion, Rollback } from "@/components/model-actions";
import { Code, PageTitle, StageChip, Unreachable, link, pct } from "@/components/ui";
import { MODEL_NAME, gatewayJson } from "@/lib/gateway";
import { production, rollbackTarget, stageName } from "@/lib/model";
import type { Gates, Model, ModelVersion, StageChange } from "@/lib/types";

// The model page (spec: dashboard "The model page shows every version and its stage").

export const metadata: Metadata = { title: "Model" };
export const dynamic = "force-dynamic";

interface Data {
  model: Model;
  versions: ModelVersion[];
  history: Record<string, StageChange[]>;
  gates: Record<string, Gates>;
}

async function load(): Promise<Data | null> {
  try {
    const [{ model }, { versions }] = await Promise.all([
      gatewayJson<{ model: Model }>(`/v1/models/${MODEL_NAME}`),
      gatewayJson<{ versions: ModelVersion[] }>(`/v1/models/${MODEL_NAME}/versions`),
    ]);
    versions.sort((a, b) => b.version - a.version);
    const history: Data["history"] = {};
    const gates: Data["gates"] = {};
    await Promise.all(
      versions.map(async (v) => {
        history[v.id] = (await gatewayJson<{ changes: StageChange[] }>(`/v1/versions/${v.id}/history`)).changes;
        if (v.stage === "MODEL_VERSION_STAGE_SHADOW") gates[v.id] = await gatewayJson<Gates>(`/v1/versions/${v.id}/gates`);
      }),
    );
    return { model, versions, history, gates };
  } catch {
    return null;
  }
}

const date = (s: string) => new Date(s).toLocaleString("en-US", { dateStyle: "medium", timeStyle: "short" });

export default async function ModelPage() {
  const data = await load();
  if (!data) {
    return (
      <>
        <PageTitle title="Model">
          <p>Every version of the model, its stage, and the promotion gates.</p>
        </PageTitle>
        <Unreachable what="the model's versions" />
      </>
    );
  }
  const { model, versions, history, gates } = data;
  const prod = production(versions);
  const target = rollbackTarget(versions);
  const shadows = versions.filter((v) => v.stage === "MODEL_VERSION_STAGE_SHADOW");

  return (
    <>
      <PageTitle title={model.name}>
        <p>
          {model.description}. Each version is an immutable artifact, content-addressed by its
          SHA-256, with its holdout metrics. A version moves through a fixed set of stages, and
          the registry refuses any move not in its transition table; the{" "}
          <Link href="/architecture#lifecycle" className={link}>architecture</Link> page draws it.
        </p>
      </PageTitle>

      <section aria-labelledby="versions-heading" className="mt-12">
        <h2 id="versions-heading" className="text-2xl font-semibold">
          Versions
        </h2>
        <table className="mt-5 w-full border-t border-ink text-left text-sm">
          <thead className="hidden md:table-header-group">
            <tr className="text-xs text-faint">
              <th scope="col" className="py-3 pr-4 font-medium">Version</th>
              <th scope="col" className="py-3 pr-4 font-medium">Stage</th>
              <th scope="col" className="py-3 pr-4 font-medium">Framework</th>
              <th scope="col" className="py-3 pr-4 font-medium">Holdout accuracy</th>
              <th scope="col" className="py-3 pr-4 font-medium">Registered</th>
              <th scope="col" className="py-3 font-medium">Stage history</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule border-t border-rule">
            {versions.map((v) => (
              <tr key={v.id} className="flex flex-col gap-1.5 py-4 md:table-row" data-version={v.version} data-stage={stageName(v.stage)}>
                <th scope="row" className="align-top text-lg font-semibold md:py-4 md:pr-4">
                  {v.version}
                  {v.id === prod?.id && <span className="sr-only"> (production)</span>}
                </th>
                <td className="align-top md:py-4 md:pr-4">
                  <StageChip stage={v.stage} />
                </td>
                <td className="align-top text-muted md:py-4 md:pr-4">{v.framework === "onnx" ? "ONNX" : "PyTorch"}</td>
                <td className="align-top md:py-4 md:pr-4">{v.metrics.accuracy !== undefined ? pct(v.metrics.accuracy, 2) : "not measured"}</td>
                <td className="align-top text-muted md:py-4 md:pr-4">{date(v.createdAt)}</td>
                <td className="align-top md:py-4">
                  <details>
                    <summary className="cursor-pointer text-code">
                      {history[v.id]?.length ?? 0} changes, last: {history[v.id]?.at(-1)?.reason || "registered"}
                    </summary>
                    <ol className="mt-2 space-y-1.5 border-l border-rule pl-3">
                      {(history[v.id] ?? []).map((c) => (
                        <li key={c.revision} className="text-muted">
                          <span className="text-ink">
                            {stageName(c.fromStage)} to {stageName(c.toStage)}
                          </span>
                          , {date(c.changedAt)}
                          {c.reason && <span className="block text-xs">“{c.reason}”</span>}
                        </li>
                      ))}
                    </ol>
                    <p className="mt-2 font-mono text-xs break-all text-faint">sha256 {v.artifactSha256}</p>
                  </details>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <div className="mt-16 grid gap-14 lg:grid-cols-2">
        <section aria-labelledby="promote-heading" className="min-w-0">
          <h2 id="promote-heading" className="text-2xl font-semibold">
            Promotion
          </h2>
          {shadows.length === 0 ? (
            <p className="mt-3 max-w-xl leading-relaxed text-muted">
              No version is in shadow, so there is nothing to promote. A new version enters shadow
              after it is registered and validated; the worker then runs it next to production on
              every job and records whether they agree.
            </p>
          ) : (
            shadows.map((v) => (
              <div key={v.id} className="mt-3">
                <p className="max-w-xl leading-relaxed text-muted">
                  Version {v.version} is in shadow next to version {prod?.version ?? "none"}. Each gate is
                  plain code in the registry (<Code>services/registry/internal/domain/promotion.go</Code>); promotion needs all of
                  them, including a recorded approval from a person.
                </p>
                <div className="mt-5">
                  <Promotion versionId={v.id} version={v.version} gates={gates[v.id]?.gates ?? []} />
                </div>
              </div>
            ))
          )}
        </section>

        <section aria-labelledby="rollback-heading" className="min-w-0">
          <h2 id="rollback-heading" className="text-2xl font-semibold">
            Rollback
          </h2>
          <div className="mt-3">
            {prod && target ? (
              <Rollback model={model.name} current={prod.version} target={target.version} />
            ) : (
              <p className="max-w-xl leading-relaxed text-muted">
                {prod
                  ? `Version ${prod.version} is in production and no earlier version was retired from production, so there is nothing to roll back to.`
                  : "No version is in production, so there is nothing to roll back."}
              </p>
            )}
          </div>
        </section>
      </div>
    </>
  );
}
