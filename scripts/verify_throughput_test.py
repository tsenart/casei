#!/usr/bin/env python3

from pathlib import Path
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
import verify_throughput as verify


def engines(row):
    return verify.required_engines(row) | {"ceiling"}


def benchmark(row, engine, speed=1000, serial=False):
    tier, name = row.split("/", 1)
    prefix = "BenchmarkIndexFold" if tier == "single" else "BenchmarkMatcher"
    separator = "#011" if serial else "\t"
    return (
        f"{prefix}/{name}/{engine}-8{separator}100{separator}100 ns/op"
        f"{separator}{speed:.2f} MB/s\n"
    )


def transcript(omit=None, samples=3, losing=None, serial=False):
    lines = []
    for row in sorted(verify.REQUIRED_ROWS):
        for engine in sorted(engines(row)):
            if (row, engine) == omit:
                continue
            speed = 900 if losing == row and engine == "candidate" else 2000 if engine == "candidate" else 1000
            for _ in range(samples):
                lines.append(benchmark(row, engine, speed, serial))
    return "".join(lines)


class VerifyThroughputTest(unittest.TestCase):
    def verify_text(self, text):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "throughput.txt"
            path.write_text(text)
            return verify.verify(path)

    def test_accepts_complete_winning_board_and_renders_markdown(self):
        medians = self.verify_text(transcript())
        self.assertEqual(34, len(medians))
        self.assertIn("PASS: 34/34 throughput rows", verify.summary(medians))
        table = verify.render(medians, "Test CPU")
        self.assertIn("#### Test CPU", table)
        self.assertIn("multi_N1_unicode_pair_miss_1_5mb", table)
        self.assertIn("**2.0**", table)
        self.assertIn("**2.00×**", table)

    def test_accepts_gce_serial_tab_encoding(self):
        self.assertEqual(34, len(self.verify_text(transcript(serial=True))))

    def test_rejects_missing_row(self):
        first = sorted(verify.REQUIRED_ROWS)[0]
        text = "".join(
            line
            for line in transcript().splitlines(keepends=True)
            if f"/{first.split('/', 1)[1]}/" not in line
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_missing_unicode_confirmation_row(self):
        row = "multi/multi_N1_unicode_pair_miss_1_5mb"
        text = "".join(
            line
            for line in transcript().splitlines(keepends=True)
            if f"/{row.split('/', 1)[1]}/" not in line
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_missing_required_engine(self):
        row = "single/log_miss_1mb"
        with self.assertRaisesRegex(verify.VerificationError, "missing engines"):
            self.verify_text(transcript(omit=(row, "vectorscan")))

    def test_rejects_wrong_sample_count(self):
        row = "single/log_miss_1mb"
        text = transcript() + benchmark(row, "candidate")
        with self.assertRaisesRegex(verify.VerificationError, "wrong sample counts"):
            self.verify_text(text)

    def test_rejects_displayed_loss(self):
        row = "multi/multi_N8_miss_log_1mb"
        with self.assertRaisesRegex(verify.VerificationError, "loses"):
            self.verify_text(transcript(losing=row))


if __name__ == "__main__":
    unittest.main()
