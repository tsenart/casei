#!/usr/bin/env python3

from pathlib import Path
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
import verify_benchmarkbar as verify


def row(
    name,
    ratio=0.5,
    entrants=None,
    competitors=None,
    candidate_bits=512,
    vectorscan_bits=512,
    vbmi=1,
    pcre2_bits=128,
    stringzilla_bits=512,
    rure_active=1,
    rure_bits=256,
    rustac_active=None,
    rustac_bits=None,
):
    multi = name.startswith("multi/")
    utf8 = verify.is_utf8_row(name)
    veloz_active = int(not multi and not utf8)
    veloz_bits = 256 if veloz_active else 0
    if rustac_active is None:
        rustac_active = int(not utf8)
    if rustac_bits is None:
        rustac_bits = 256 if rustac_active else 0
    go_ac_active = int(multi and not utf8)
    if competitors is None:
        competitors = 4 + rure_active + veloz_active + rustac_active
    if entrants is None:
        entrants = 1 + competitors + go_ac_active
    multi_metrics = ""
    if multi:
        multi_metrics = f"{go_ac_active} go_ac_active 0 go_ac_vector_bits "
    return (
        f"BenchmarkBar/{name}-8 30 100 ns/op "
        f"{ratio} x_vs_best {competitors} competitors {entrants} entrants "
        f"1 candidate_active {candidate_bits} candidate_vector_bits "
        f"1 regexp_active 0 regexp_vector_bits "
        f"1 pcre2_active {pcre2_bits} pcre2_vector_bits "
        f"{rure_active} rure_active {rure_bits} rure_vector_bits "
        f"1 vectorscan_active {vectorscan_bits} vectorscan_vector_bits "
        f"{vbmi} vectorscan_vbmi "
        f"1 stringzilla_active {stringzilla_bits} stringzilla_vector_bits "
        f"{veloz_active} veloz_active {veloz_bits} veloz_vector_bits "
        f"{rustac_active} rustac_active {rustac_bits} rustac_vector_bits "
        f"{multi_metrics}\n"
    )


def transcript(**override):
    lines = []
    for index, name in enumerate(sorted(verify.REQUIRED_ROWS)):
        values = override if index == 0 else {}
        for _ in range(3):
            lines.append(row(name, **values))
    return "".join(lines)


class VerifyBenchmarkBarTest(unittest.TestCase):
    def verify_text(self, text, required_rows=verify.REQUIRED_ROWS):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bar.txt"
            path.write_text(text)
            return verify.verify(path, required_rows=required_rows)

    def test_accepts_complete_winning_full_width_board(self):
        summary = self.verify_text(transcript())
        self.assertIn("PASS: 38/38 rows", summary)
        self.assertIn("casei=512-bit", summary)

    def test_accepts_historical_36_row_board(self):
        text = "".join(
            row(name)
            for name in sorted(verify.HISTORICAL_REQUIRED_ROWS)
            for _ in range(3)
        )
        summary = self.verify_text(text, verify.HISTORICAL_REQUIRED_ROWS)
        self.assertIn("PASS: 36/36 rows", summary)

    def test_rejects_losing_sample(self):
        with self.assertRaisesRegex(verify.VerificationError, "loses"):
            self.verify_text(transcript(ratio=1.0))

    def test_rejects_handicapped_vectorscan(self):
        with self.assertRaisesRegex(verify.VerificationError, "vectorscan_vector_bits"):
            self.verify_text(transcript(vectorscan_bits=256))

    def test_rejects_handicapped_pcre2(self):
        with self.assertRaisesRegex(verify.VerificationError, "pcre2_vector_bits"):
            self.verify_text(transcript(pcre2_bits=0))

    def test_rejects_dropped_rure(self):
        with self.assertRaisesRegex(verify.VerificationError, "rure_active"):
            self.verify_text(transcript(rure_active=0, rure_bits=0))

    def test_rejects_dropped_rure_on_single_row(self):
        text = "".join(
            row(name, rure_active=0, rure_bits=0)
            if name.startswith("single/")
            else row(name)
            for name in sorted(verify.REQUIRED_ROWS)
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "rure_active"):
            self.verify_text(text)

    def test_rejects_rustac_on_utf8_row(self):
        text = "".join(
            row(name, rustac_active=1, rustac_bits=256)
            if verify.is_utf8_row(name)
            else row(name)
            for name in sorted(verify.REQUIRED_ROWS)
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "rustac_active"):
            self.verify_text(text)

    def test_rure_width_is_diagnostic(self):
        summary = self.verify_text(transcript(rure_bits=0))
        self.assertIn("PASS", summary)

    def test_rejects_dropped_rustac(self):
        text = "".join(
            row(name, rustac_active=0, rustac_bits=0)
            if name.startswith("multi/") and not verify.is_utf8_row(name)
            else row(name)
            for name in sorted(verify.REQUIRED_ROWS)
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "rustac_active"):
            self.verify_text(text)

    def test_rejects_dropped_rustac_on_single_row(self):
        text = "".join(
            row(name, rustac_active=0, rustac_bits=0)
            if name.startswith("single/") and not verify.is_utf8_row(name)
            else row(name)
            for name in sorted(verify.REQUIRED_ROWS)
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "rustac_active"):
            self.verify_text(text)

    def test_rejects_single_row_without_rustac_metrics(self):
        name = "single/log_miss_1mb"
        text = "".join(
            line.split(" 1 rustac_active")[0] + "\n" if f"/{name.split('/', 1)[1]}-" in line else line
            for line in transcript().splitlines(keepends=True)
        )
        with self.assertRaisesRegex(verify.VerificationError, "missing rustac_active"):
            self.verify_text(text)

    def test_rustac_width_is_diagnostic(self):
        text = "".join(
            row(name, rustac_bits=0)
            for name in sorted(verify.REQUIRED_ROWS)
            for _ in range(3)
        )
        self.assertIn("PASS", self.verify_text(text))

    def test_rejects_unmeasured_row(self):
        with self.assertRaisesRegex(verify.VerificationError, "unmeasured"):
            self.verify_text(transcript(entrants=1))

    def test_rejects_missing_row(self):
        text = "".join(
            row(name)
            for name in sorted(verify.REQUIRED_ROWS)[:-1]
            for _ in range(3)
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_missing_unicode_confirmation_row(self):
        row_name = "multi/multi_N1_unicode_pair_miss_1_5mb"
        text = "".join(
            line
            for line in transcript().splitlines(keepends=True)
            if f"/{row_name.split('/', 1)[1]}-" not in line
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_missing_complete_triple_row(self):
        row_name = "multi/multi_forms4_complete_triple_miss_1mb"
        text = "".join(
            line
            for line in transcript().splitlines(keepends=True)
            if f"/{row_name.split('/', 1)[1]}-" not in line
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_missing_raw_transition_row(self):
        row_name = "multi/multi_N5_raw_transition_late_hit_5mb"
        text = "".join(
            line
            for line in transcript().splitlines(keepends=True)
            if f"/{row_name.split('/', 1)[1]}-" not in line
        )
        with self.assertRaisesRegex(verify.VerificationError, "row inventory differs"):
            self.verify_text(text)

    def test_rejects_wrong_sample_count(self):
        with self.assertRaisesRegex(verify.VerificationError, "wrong sample counts"):
            self.verify_text(
                transcript() + row(sorted(verify.REQUIRED_ROWS)[0])
            )


if __name__ == "__main__":
    unittest.main()
