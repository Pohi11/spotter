"""`spotter-drift-experiment`: measure how quickly the drift monitor notices degraded traffic,
and how often it raises false alarms on clean traffic.

For each degradation and strength: send clean holdout tiles until the version is not
drifting and its window is clean, then send degraded tiles until DriftDetected arrives (or a
cap), counting degraded tiles sent. Separately, send a long run of clean tiles and count
DriftDetected events. Writes benchmarks/<date>-drift-detection.{json,md}.

Configuration: GATEWAY_GRPC_ADDR (localhost:9091), NATS_URL, MODEL_NAME (eurosat-classifier),
DATA_DIR (data), SEED (42), CLEAN_EVENTS (3000), CAP (1000), CONCURRENCY (4),
BENCH_DIR (benchmarks).
"""

import asyncio
import json
import os
import random
from datetime import UTC, datetime
from pathlib import Path

import grpc
import nats
from spotter.v1 import inference_pb2 as ipb
from spotter.v1 import inference_pb2_grpc as irpc
from spotter.v1 import monitor_pb2 as mpb

from spotter_trainer import data
from spotter_trainer.degrade import KINDS, degrade_jpeg
from spotter_trainer.splits import split

STRENGTHS = (0.25, 0.5, 1.0)
WINDOW = 200


class Experiment:
    def __init__(self, stub, model: str, clean: list[bytes], concurrency: int) -> None:
        self.stub, self.model, self.clean, self.concurrency = stub, model, clean, concurrency
        self.detected = asyncio.Event()
        self.resolved = asyncio.Event()
        self.detections = 0
        self.last_detected: mpb.DriftDetected | None = None

    async def on_drift(self, msg) -> None:
        if ".detected." in msg.subject:
            self.detections += 1
            self.last_detected = mpb.DriftDetected.FromString(msg.data)
            self.detected.set()
        else:
            self.resolved.set()

    async def send(self, images: list[bytes], n: int, stop: asyncio.Event | None = None) -> int:
        """Submit up to n images from `concurrency` clients; return how many were sent."""
        sent = 0
        lock = asyncio.Lock()

        async def client(offset: int) -> None:
            nonlocal sent
            i = offset
            while True:
                async with lock:
                    if sent >= n or (stop is not None and stop.is_set()):
                        return
                    sent += 1
                await self.stub.SubmitInference(
                    ipb.SubmitInferenceRequest(model_name=self.model, image=images[i % len(images)])
                )
                i += self.concurrency
                await asyncio.sleep(0.015)  # about the worker's pace, so the queue stays short

        await asyncio.gather(*(client(k) for k in range(self.concurrency)))
        return sent

    async def make_clean(self) -> None:
        """Fill the window with clean traffic; wait for DriftResolved if drifting."""
        self.resolved.clear()
        await self.send(self.clean, WINDOW + 100)
        if self.detected.is_set():
            await self.send(self.clean, 2000, stop=self.resolved)
            await asyncio.sleep(1)
        self.detected.clear()


async def amain() -> None:
    e = os.environ.get
    model = e("MODEL_NAME", "eurosat-classifier")
    images_dir = data.download(Path(e("DATA_DIR", "data")))
    s = split(data.list_images(images_dir), seed=int(e("SEED", "42")))
    holdout = list(s.holdout)
    random.Random(0).shuffle(holdout)
    clean = [(images_dir / p).read_bytes() for p in holdout[:400]]

    channel = grpc.aio.insecure_channel(e("GATEWAY_GRPC_ADDR", "localhost:9091"))
    exp = Experiment(irpc.InferenceServiceStub(channel), model, clean, int(e("CONCURRENCY", "4")))
    nc = await nats.connect(e("NATS_URL", "nats://localhost:4222"))
    await nc.subscribe(f"monitor.drift.*.{model}", cb=exp.on_drift)

    await exp.make_clean()
    before = exp.detections
    clean_events = int(e("CLEAN_EVENTS", "3000"))
    await exp.send(clean, clean_events)
    await asyncio.sleep(2)
    false_alarms = exp.detections - before
    print(f"clean: {clean_events} events, {false_alarms} false alarms", flush=True)

    cap = int(e("CAP", "1000"))
    rows = []
    for kind in KINDS:
        for strength in STRENGTHS:
            degraded = [degrade_jpeg(b, kind, strength, seed=i) for i, b in enumerate(clean)]
            await exp.make_clean()
            sent = await exp.send(degraded, cap, stop=exp.detected)
            await asyncio.sleep(0.5)
            found = exp.detected.is_set()
            feats = sorted(f.feature for f in exp.last_detected.features) if found else []
            rows.append(
                {
                    "degradation": kind,
                    "strength": strength,
                    "detected": found,
                    "degraded_events_until_detection": sent if found else None,
                    "drifted_features": feats,
                }
            )
            print(
                f"{kind} {strength}: {'detected after ' + str(sent) if found else 'not detected'}"
                f" {feats}",
                flush=True,
            )

    date = datetime.now(UTC).strftime("%Y-%m-%d")
    report = {
        "date": date,
        "model": model,
        "window": WINDOW,
        "evaluate_every": 50,
        "psi_enter": 0.2,
        "psi_exit": 0.1,
        "clean_events": clean_events,
        "false_alarms": false_alarms,
        "cap": cap,
        "conditions": rows,
    }
    write_report(Path(e("BENCH_DIR", "benchmarks")), report)
    await nc.close()
    await channel.close()


def write_report(out: Path, report: dict) -> None:
    out.mkdir(parents=True, exist_ok=True)
    date, cap = report["date"], report["cap"]
    (out / f"{date}-drift-detection.json").write_text(
        json.dumps(report, indent=2), encoding="utf-8"
    )
    lines = [
        f"# Drift detection, {date}",
        "",
        f"Window {WINDOW} events, evaluated every 50; drift at PSI ≥ 0.2, resolved below 0.1.",
        f"Clean traffic: **{report['false_alarms']} false alarms in {report['clean_events']} "
        f"events** ({report['clean_events'] // 50} evaluations).",
        "",
        "| Degradation | Strength | Detected | Degraded events to detect | Drifted features |",
        "|---|---|---|---|---|",
    ]
    for r in report["conditions"]:
        n = r["degraded_events_until_detection"]
        lines.append(
            f"| {r['degradation']} | {r['strength']} | {'yes' if r['detected'] else 'no'} | "
            f"{n if n is not None else f'> {cap}'} | {', '.join(r['drifted_features'])} |"
        )
    (out / f"{date}-drift-detection.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> None:
    asyncio.run(amain())


if __name__ == "__main__":
    main()
