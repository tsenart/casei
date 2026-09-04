#include "textflag.h"

// rootSkip32 scans AVX2-sized blocks. Bit i of the combined mask marks a byte
// that either has its high bit set or equals target after ORing fold.
TEXT ·rootSkip32(SB), NOSPLIT, $0-40
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ target+16(FP), CX
	VMOVQ CX, X1
	VPBROADCASTQ X1, Y1
	MOVQ fold+24(FP), CX
	VMOVQ CX, X2
	VPBROADCASTQ X2, Y2

loop32:
	CMPQ DX, $32
	JL done32
	VMOVDQU (AX), Y0
	VPMOVMSKB Y0, CX
	VPOR Y2, Y0, Y0
	VPCMPEQB Y1, Y0, Y0
	VPMOVMSKB Y0, R8
	ORL R8, CX
	TESTL CX, CX
	JNZ stop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP loop32
stop32:
	BSFL CX, CX
	ADDQ CX, BX
done32:
	MOVQ BX, ret+32(FP)
	VZEROUPPER
	RET

// rootSkip64 is the AVX-512 BW equivalent. VPMOVB2M writes the high-bit mask
// directly to an opmask; VPCMPEQB contributes the root-byte matches.
TEXT ·rootSkip64(SB), NOSPLIT, $0-40
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ target+16(FP), CX
	VPBROADCASTB CX, K1, Z1
	MOVQ fold+24(FP), CX
	VPBROADCASTB CX, K1, Z2

rootdouble64:
	CMPQ DX, $128
	JL loop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 64(AX), K1, Z3
	VPMOVB2M Z0, K2
	VPMOVB2M Z3, K4
	VPORQ Z2, Z0, K1, Z0
	VPORQ Z2, Z3, K1, Z3
	VPCMPEQB Z1, Z0, K1, K3
	VPCMPEQB Z1, Z3, K1, K5
	KORQ K2, K3, K2
	KORQ K4, K5, K4
	KORTESTQ K2, K4
	JNE rootdoublestop64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP rootdouble64
rootdoublestop64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ stop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP done64
loop64:
	CMPQ DX, $64
	JL done64
	VMOVDQU8 (AX), K1, Z0
	VPMOVB2M Z0, K2
	VPORQ Z2, Z0, K1, Z0
	VPCMPEQB Z1, Z0, K1, K3
	KORQ K2, K3, K2
	KTESTQ K2, K2
	JNE stop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP loop64
stop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
done64:
	MOVQ BX, ret+32(FP)
	VZEROUPPER
	RET

// literalSkip32 is rootSkip32 without the high-byte stop mask. It is safe only
// for a fixed ASCII anchor, whose callers may skip UTF-8 and malformed bytes.
TEXT ·literalSkip32(SB), NOSPLIT, $0-40
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ target+16(FP), CX
	VMOVQ CX, X1
	VPBROADCASTQ X1, Y1
	MOVQ fold+24(FP), CX
	VMOVQ CX, X2
	VPBROADCASTQ X2, Y2
literalloop32:
	CMPQ DX, $32
	JL literaldone32
	VMOVDQU (AX), Y0
	VPOR Y2, Y0, Y0
	VPCMPEQB Y1, Y0, Y0
	VPMOVMSKB Y0, CX
	TESTL CX, CX
	JNZ literalstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP literalloop32
literalstop32:
	BSFL CX, CX
	ADDQ CX, BX
literaldone32:
	MOVQ BX, ret+32(FP)
	VZEROUPPER
	RET

// literalSkip64 is the AVX-512 BW form of literalSkip32.
TEXT ·literalSkip64(SB), NOSPLIT, $0-40
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ target+16(FP), CX
	VPBROADCASTB CX, K1, Z1
	MOVQ fold+24(FP), CX
	VPBROADCASTB CX, K1, Z2
literaldouble64:
	CMPQ DX, $128
	JL literalloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 64(AX), K1, Z3
	VPORQ Z2, Z0, K1, Z0
	VPORQ Z2, Z3, K1, Z3
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z1, Z3, K1, K3
	KORTESTQ K2, K3
	JNE literaldoublestop64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP literaldouble64
literaldoublestop64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ literalstop64
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP literaldone64
literalloop64:
	CMPQ DX, $64
	JL literaldone64
	VMOVDQU8 (AX), K1, Z0
	VPORQ Z2, Z0, K1, Z0
	VPCMPEQB Z1, Z0, K1, K2
	KTESTQ K2, K2
	JNE literalstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP literalloop64
literalstop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
literaldone64:
	MOVQ BX, ret+32(FP)
	VZEROUPPER
	RET

// literalSkipExact64 is the exact-byte specialization of literalSkip64. Fixed
// literal anchors need no fold vector. Eight unmasked memory-source compares
// keep independent cache lines in flight before the first ordered mask test.
TEXT ·literalSkipExact64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ target+16(FP), CX
	VPBROADCASTB CX, Z1
	MOVQ DX, R8
	SHRQ $9, R8
	JZ literalexactquad64

literalexactloop64:
	VPCMPEQB (AX), Z1, K0
	VPCMPEQB 64(AX), Z1, K1
	VPCMPEQB 128(AX), Z1, K2
	VPCMPEQB 192(AX), Z1, K3
	VPCMPEQB 256(AX), Z1, K4
	VPCMPEQB 320(AX), Z1, K5
	VPCMPEQB 384(AX), Z1, K6
	VPCMPEQB 448(AX), Z1, K7
	KORTESTQ K0, K1
	JNE literalexactfirstpair64
	KORTESTQ K2, K3
	JNE literalexactsecondpair64
	KORTESTQ K4, K5
	JNE literalexactthirdpair64
	KORTESTQ K6, K7
	JNE literalexactfourthpair64
	ADDQ $512, AX
	DECQ R8
	JNZ literalexactloop64
	ANDQ $511, DX
	JMP literalexactquad64

literalexactfirstpair64:
	KTESTQ K0, K0
	JNE literalexactblock064
	KMOVQ K1, CX
	BSFQ CX, CX
	ADDQ $64, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactsecondpair64:
	KTESTQ K2, K2
	JNE literalexactblock264
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $192, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactthirdpair64:
	KTESTQ K4, K4
	JNE literalexactblock464
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $320, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactfourthpair64:
	KTESTQ K6, K6
	JNE literalexactblock664
	KMOVQ K7, CX
	BSFQ CX, CX
	ADDQ $448, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactblock064:
	KMOVQ K0, CX
	BSFQ CX, CX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactblock264:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ $128, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactblock464:
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $256, AX
	ADDQ CX, AX
	JMP literalexactdone64
literalexactblock664:
	KMOVQ K6, CX
	BSFQ CX, CX
	ADDQ $384, AX
	ADDQ CX, AX
	JMP literalexactdone64

literalexactquad64:
	CMPQ DX, $256
	JL literalexactdouble64
	VPCMPEQB (AX), Z1, K0
	VPCMPEQB 64(AX), Z1, K1
	VPCMPEQB 128(AX), Z1, K2
	VPCMPEQB 192(AX), Z1, K3
	KORTESTQ K0, K1
	JNE literalexactfirstpair64
	KORTESTQ K2, K3
	JNE literalexactsecondpair64
	ADDQ $256, AX
	SUBQ $256, DX

literalexactdouble64:
	CMPQ DX, $128
	JL literalexactsingle64
	VPCMPEQB (AX), Z1, K0
	VPCMPEQB 64(AX), Z1, K1
	KORTESTQ K0, K1
	JNE literalexactfirstpair64
	ADDQ $128, AX
	SUBQ $128, DX

literalexactsingle64:
	CMPQ DX, $64
	JL literalexactdone64
	VPCMPEQB (AX), Z1, K0
	KTESTQ K0, K0
	JNE literalexactblock064
	ADDQ $64, AX
literalexactdone64:
	SUBQ ptr+0(FP), AX
	MOVQ AX, ret+24(FP)
	VZEROUPPER
	RET

// runMask32 returns one equality bit per byte for a repeated-token block.
TEXT ·runMask32(SB), NOSPLIT, $0-28
	MOVQ ptr+0(FP), AX
	MOVQ target+8(FP), CX
	VMOVQ CX, X1
	VPBROADCASTQ X1, Y1
	MOVQ fold+16(FP), CX
	VMOVQ CX, X2
	VPBROADCASTQ X2, Y2
	VMOVDQU (AX), Y0
	VPOR Y2, Y0, Y0
	VPCMPEQB Y1, Y0, Y0
	VPMOVMSKB Y0, AX
	MOVL AX, ret+24(FP)
	VZEROUPPER
	RET

// runMask64 is the AVX-512 BW form of runMask32.
TEXT ·runMask64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ target+8(FP), CX
	VPBROADCASTB CX, K1, Z1
	MOVQ fold+16(FP), CX
	VPBROADCASTB CX, K1, Z2
	VMOVDQU8 (AX), K1, Z0
	VPORQ Z2, Z0, K1, Z0
	VPCMPEQB Z1, Z0, K1, K2
	KMOVQ K2, AX
	MOVQ AX, ret+24(FP)
	VZEROUPPER
	RET

// probeSkip32 intersects three fixed needle positions for every candidate
// start in an AVX2 block. asciiProbe stores bytes at 0..3 and offsets at
// 8, 16, and 24.
TEXT ·probeSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVBLZX 0(SI), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 1(SI), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
	MOVBLZX 2(SI), R8
	VMOVQ R8, X3
	VPBROADCASTB X3, Y3
	MOVQ $0x2020202020202020, R8
	VMOVQ R8, X8
	VPBROADCASTQ X8, Y8
	MOVBLZX 3(SI), R11

probeloop32:
	CMPQ DX, $32
	JL probedone32
	MOVQ 8(SI), R8
	VMOVDQU (AX)(R8*1), Y0
	MOVQ 16(SI), R8
	VMOVDQU (AX)(R8*1), Y4
	MOVQ 24(SI), R8
	VMOVDQU (AX)(R8*1), Y5
	TESTQ $1, R11
	JZ proberawfirst32
	VPOR Y8, Y0, Y0
proberawfirst32:
	TESTQ $2, R11
	JZ proberawsecond32
	VPOR Y8, Y4, Y4
proberawsecond32:
	TESTQ $4, R11
	JZ proberawthird32
	VPOR Y8, Y5, Y5
proberawthird32:
	VPCMPEQB Y1, Y0, Y0
	VPCMPEQB Y2, Y4, Y4
	VPCMPEQB Y3, Y5, Y5
	VPMOVMSKB Y0, CX
	VPMOVMSKB Y4, R8
	ANDL R8, CX
	VPMOVMSKB Y5, R8
	ANDL R8, CX
	TESTL CX, CX
	JNZ probestop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP probeloop32
probestop32:
	BSFL CX, CX
	ADDQ CX, BX
probedone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// probeSkip64 is the AVX-512 BW form of probeSkip32.
TEXT ·probeSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8
	MOVBLZX 3(SI), R11

probedouble64:
	CMPQ DX, $128
	JL probeloop64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 64(AX)(R8*1), K1, Z9
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	VMOVDQU8 64(AX)(R8*1), K1, Z10
	MOVQ 24(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z5
	VMOVDQU8 64(AX)(R8*1), K1, Z11
	TESTQ $1, R11
	JZ proberawfirstdouble64
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z8, Z9, K1, Z9
proberawfirstdouble64:
	TESTQ $2, R11
	JZ proberawseconddouble64
	VPORQ Z8, Z4, K1, Z4
	VPORQ Z8, Z10, K1, Z10
proberawseconddouble64:
	TESTQ $4, R11
	JZ proberawthirddouble64
	VPORQ Z8, Z5, K1, Z5
	VPORQ Z8, Z11, K1, Z11
proberawthirddouble64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	VPCMPEQB Z3, Z5, K1, K4
	KANDQ K2, K3, K2
	KANDQ K2, K4, K2
	VPCMPEQB Z1, Z9, K1, K5
	VPCMPEQB Z2, Z10, K1, K6
	VPCMPEQB Z3, Z11, K1, K7
	KANDQ K5, K6, K5
	KANDQ K5, K7, K5
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ probestop64
	KMOVQ K5, CX
	TESTQ CX, CX
	JNZ probesecondstop64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP probedouble64
probesecondstop64:
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP probedone64

probeloop64:
	CMPQ DX, $64
	JL probedone64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	MOVQ 24(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z5
	TESTQ $1, R11
	JZ proberawfirst64
	VPORQ Z8, Z0, K1, Z0
proberawfirst64:
	TESTQ $2, R11
	JZ proberawsecond64
	VPORQ Z8, Z4, K1, Z4
proberawsecond64:
	TESTQ $4, R11
	JZ proberawthird64
	VPORQ Z8, Z5, K1, Z5
proberawthird64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	VPCMPEQB Z3, Z5, K1, K4
	KANDQ K2, K3, K2
	KANDQ K2, K4, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ probestop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP probeloop64
probestop64:
	BSFQ CX, CX
	ADDQ CX, BX
probedone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiOnlyProbeSkip64 combines the ordinary three-byte probe with an
// high-byte detector. Its caller uses a high-byte stop to resume the full
// Unicode transition path; otherwise the all-ASCII spelling is exact.
TEXT ·asciiOnlyProbeSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8
	MOVBLZX 3(SI), R11
	MOVQ 16(SI), R10
	CMPQ R10, 24(SI)
	JE asciionlypairdouble64

asciionlyprobedouble64:
	CMPQ DX, $128
	JL asciionlyprobeloop64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 64(AX)(R8*1), K1, Z9
	VPMOVB2M Z0, K7
	VPMOVB2M Z9, K6
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	VMOVDQU8 64(AX)(R8*1), K1, Z10
	MOVQ 24(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z5
	VMOVDQU8 64(AX)(R8*1), K1, Z11
	TESTQ $1, R11
	JZ asciionlyproberawfirstdouble64
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z8, Z9, K1, Z9
asciionlyproberawfirstdouble64:
	TESTQ $2, R11
	JZ asciionlyproberawseconddouble64
	VPORQ Z8, Z4, K1, Z4
	VPORQ Z8, Z10, K1, Z10
asciionlyproberawseconddouble64:
	TESTQ $4, R11
	JZ asciionlyproberawthirddouble64
	VPORQ Z8, Z5, K1, Z5
	VPORQ Z8, Z11, K1, Z11
asciionlyproberawthirddouble64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	VPCMPEQB Z3, Z5, K1, K4
	KANDQ K2, K3, K2
	KANDQ K2, K4, K2
	KORQ K7, K2, K2
	VPCMPEQB Z1, Z9, K1, K5
	VPCMPEQB Z2, Z10, K1, K3
	VPCMPEQB Z3, Z11, K1, K4
	KANDQ K5, K3, K5
	KANDQ K5, K4, K5
	KORQ K6, K5, K5
	KORTESTQ K2, K5
	JNE asciionlyprobedoublestop64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP asciionlyprobedouble64
asciionlyprobedoublestop64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciionlyprobestop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciionlyprobedone64

asciionlyprobeloop64:
	CMPQ DX, $64
	JL asciionlyprobedone64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VPMOVB2M Z0, K7
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	MOVQ 24(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z5
	TESTQ $1, R11
	JZ asciionlyproberawfirst64
	VPORQ Z8, Z0, K1, Z0
asciionlyproberawfirst64:
	TESTQ $2, R11
	JZ asciionlyproberawsecond64
	VPORQ Z8, Z4, K1, Z4
asciionlyproberawsecond64:
	TESTQ $4, R11
	JZ asciionlyproberawthird64
	VPORQ Z8, Z5, K1, Z5
asciionlyproberawthird64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	VPCMPEQB Z3, Z5, K1, K4
	KANDQ K2, K3, K2
	KANDQ K2, K4, K2
	KORQ K7, K2, K2
	KTESTQ K2, K2
	JNE asciionlyprobestop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciionlyprobeloop64
asciionlyprobestop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
asciionlyprobedone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// The structured ASCII-only form repeats its final probe position. Once the
// high-byte guard has established ASCII, compare that boundary pair directly
// instead of loading and intersecting the same displaced stream twice.
asciionlypairdouble64:
	CMPQ DX, $128
	JL asciionlypairloop64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 64(AX)(R8*1), K1, Z9
	VPMOVB2M Z0, K7
	VPMOVB2M Z9, K6
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	VMOVDQU8 64(AX)(R8*1), K1, Z10
	TESTQ $1, R11
	JZ asciionlypairrawfirstdouble64
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z8, Z9, K1, Z9
asciionlypairrawfirstdouble64:
	TESTQ $2, R11
	JZ asciionlypairrawseconddouble64
	VPORQ Z8, Z4, K1, Z4
	VPORQ Z8, Z10, K1, Z10
asciionlypairrawseconddouble64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	KANDQ K2, K3, K2
	KORQ K7, K2, K2
	VPCMPEQB Z1, Z9, K1, K5
	VPCMPEQB Z2, Z10, K1, K3
	KANDQ K5, K3, K5
	KORQ K6, K5, K5
	KORTESTQ K2, K5
	JNE asciionlypairfounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP asciionlypairdouble64
asciionlypairfounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciionlypairstop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciionlypairdone64

asciionlypairloop64:
	CMPQ DX, $64
	JL asciionlypairdone64
	MOVQ 8(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VPMOVB2M Z0, K7
	MOVQ 16(SI), R8
	VMOVDQU8 (AX)(R8*1), K1, Z4
	TESTQ $1, R11
	JZ asciionlypairrawfirst64
	VPORQ Z8, Z0, K1, Z0
asciionlypairrawfirst64:
	TESTQ $2, R11
	JZ asciionlypairrawsecond64
	VPORQ Z8, Z4, K1, Z4
asciionlypairrawsecond64:
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z4, K1, K3
	KANDQ K2, K3, K2
	KORQ K7, K2, K2
	KTESTQ K2, K2
	JNE asciionlypairstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciionlypairloop64
asciionlypairstop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
asciionlypairdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairSkip32 scans overlapping AVX2 blocks for the two-byte root prefix. The
// second load lets the final lane examine the following byte as well.
TEXT ·pairSkip32(SB), NOSPLIT, $0-56
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ first+16(FP), CX
	VMOVQ CX, X1
	VPBROADCASTQ X1, Y1
	MOVQ firstFold+24(FP), CX
	VMOVQ CX, X2
	VPBROADCASTQ X2, Y2
	MOVQ second+32(FP), CX
	VMOVQ CX, X3
	VPBROADCASTQ X3, Y3
	MOVQ secondFold+40(FP), CX
	VMOVQ CX, X4
	VPBROADCASTQ X4, Y4

pairloop32:
	CMPQ DX, $33
	JL pairdone32
	VMOVDQU (AX), Y0
	VMOVDQU 1(AX), Y5
	VPMOVMSKB Y0, CX
	VPMOVMSKB Y5, R8
	ORL R8, CX
	VPOR Y2, Y0, Y0
	VPOR Y4, Y5, Y5
	VPCMPEQB Y1, Y0, Y0
	VPCMPEQB Y3, Y5, Y5
	VPMOVMSKB Y0, R8
	VPMOVMSKB Y5, R9
	ANDL R9, R8
	ORL R8, CX
	TESTL CX, CX
	JNZ pairstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP pairloop32
pairstop32:
	BSFL CX, CX
	ADDQ CX, BX
pairdone32:
	MOVQ BX, ret+48(FP)
	VZEROUPPER
	RET

// pairSkip64 is the AVX-512 BW version of pairSkip32.
TEXT ·pairSkip64(SB), NOSPLIT, $0-56
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ first+16(FP), CX
	VPBROADCASTB CX, K1, Z1
	MOVQ firstFold+24(FP), CX
	VPBROADCASTB CX, K1, Z2
	MOVQ second+32(FP), CX
	VPBROADCASTB CX, K1, Z3
	MOVQ secondFold+40(FP), CX
	VPBROADCASTB CX, K1, Z4
	// Keep a start-of-haystack hit on the original one-block latency path.
	JMP pairloop64

// Two independent blocks amortize the stop branch and let the byte-mask,
// fold, and comparison chains overlap on the ordinary root-to-root miss.
pairdouble64:
	CMPQ DX, $129
	JL pairloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VMOVDQU8 64(AX), K1, Z6
	VMOVDQU8 65(AX), K1, Z7
	VPMOVB2M Z0, K2
	VPMOVB2M Z5, K3
	KORQ K2, K3, K2
	VPMOVB2M Z6, K4
	VPMOVB2M Z7, K5
	KORQ K4, K5, K4
	VPORQ Z2, Z0, K1, Z0
	VPORQ Z4, Z5, K1, Z5
	VPCMPEQB Z1, Z0, K1, K3
	VPCMPEQB Z3, Z5, K1, K5
	KANDQ K3, K5, K3
	KORQ K2, K3, K2
	VPORQ Z2, Z6, K1, Z6
	VPORQ Z4, Z7, K1, Z7
	VPCMPEQB Z1, Z6, K1, K3
	VPCMPEQB Z3, Z7, K1, K5
	KANDQ K3, K5, K3
	KORQ K4, K3, K4
	KORTESTQ K2, K4
	JNE pairfounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairdouble64
pairfounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairstop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP pairdone64

pairloop64:
	CMPQ DX, $65
	JL pairdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VPMOVB2M Z0, K2
	VPMOVB2M Z5, K3
	KORQ K2, K3, K2
	VPORQ Z2, Z0, K1, Z0
	VPORQ Z4, Z5, K1, Z5
	VPCMPEQB Z1, Z0, K1, K4
	VPCMPEQB Z3, Z5, K1, K5
	KANDQ K4, K5, K4
	KORQ K2, K4, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairdouble64
pairstop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairdone64:
	MOVQ BX, ret+48(FP)
	VZEROUPPER
	RET

// pairSetSkip32 specializes the common two-raw-UTF-8-root case. Its four
// broadcasts stay live across blocks, unlike the bounded general filter loop.
TEXT ·pairSetSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVBLZX 16(SI), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 17(SI), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
	MOVBLZX 20(SI), R8
	VMOVQ R8, X3
	VPBROADCASTB X3, Y3
	MOVBLZX 21(SI), R8
	VMOVQ R8, X4
	VPBROADCASTB X4, Y4

pairsetloop32:
	CMPQ DX, $33
	JL pairsetdone32
	VMOVDQU (AX), Y0
	VMOVDQU 1(AX), Y5
	XORL CX, CX
	VPCMPEQB Y1, Y0, Y6
	VPCMPEQB Y2, Y5, Y7
	VPMOVMSKB Y6, R8
	VPMOVMSKB Y7, R9
	ANDL R9, R8
	ORL R8, CX
	VPCMPEQB Y3, Y0, Y6
	VPCMPEQB Y4, Y5, Y7
	VPMOVMSKB Y6, R8
	VPMOVMSKB Y7, R9
	ANDL R9, R8
	ORL R8, CX
	TESTL CX, CX
	JNZ pairsetstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP pairsetloop32
pairsetstop32:
	BSFL CX, CX
	ADDQ CX, BX
pairsetdone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairSetSkip64 is the AVX-512 BW form of pairSetSkip32.
TEXT ·pairSetSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 16(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 17(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 20(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 21(SI), R8
	VPBROADCASTB R8, K1, Z4

pairsetdouble64:
	CMPQ DX, $129
	JL pairsetloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VMOVDQU8 64(AX), K1, Z6
	VMOVDQU8 65(AX), K1, Z7
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z0, K1, K3
	VPCMPEQB Z4, Z5, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	VPCMPEQB Z1, Z6, K1, K4
	VPCMPEQB Z2, Z7, K1, K5
	KANDQ K4, K5, K4
	VPCMPEQB Z3, Z6, K1, K5
	VPCMPEQB Z4, Z7, K1, K6
	KANDQ K5, K6, K5
	KORQ K4, K5, K4
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairsetstop64
	KMOVQ K4, CX
	TESTQ CX, CX
	JNZ pairsetsecondstop64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairsetdouble64
pairsetsecondstop64:
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP pairsetdone64

pairsetloop64:
	CMPQ DX, $65
	JL pairsetdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z0, K1, K3
	VPCMPEQB Z4, Z5, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairsetstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairsetloop64
pairsetstop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairsetdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairPairSkip64 intersects two raw pair sets at a fixed positive byte offset.
// pairPairFilter stores the two primary pairs at bytes 0..3, the two
// confirmation pairs at 4..7, and the confirmation offset at byte 8.
TEXT ·pairPairSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 3(SI), R8
	VPBROADCASTB R8, K1, Z4
	MOVBLZX 4(SI), R8
	VPBROADCASTB R8, K1, Z5
	MOVBLZX 5(SI), R8
	VPBROADCASTB R8, K1, Z6
	MOVBLZX 6(SI), R8
	VPBROADCASTB R8, K1, Z7
	MOVBLZX 7(SI), R8
	VPBROADCASTB R8, K1, Z8
	MOVBLZX 8(SI), R8
pairpairdouble64:
	CMPQ DX, $128
	JL pairpairloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VMOVDQU8 64(AX), K1, Z12
	VMOVDQU8 65(AX), K1, Z13
	VMOVDQU8 64(AX)(R8*1), K1, Z14
	VMOVDQU8 65(AX)(R8*1), K1, Z15
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z9, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z0, K1, K3
	VPCMPEQB Z4, Z9, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	VPCMPEQB Z5, Z10, K1, K3
	VPCMPEQB Z6, Z11, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z7, Z10, K1, K4
	VPCMPEQB Z8, Z11, K1, K6
	KANDQ K4, K6, K4
	KORQ K3, K4, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z1, Z12, K1, K5
	VPCMPEQB Z2, Z13, K1, K3
	KANDQ K5, K3, K5
	VPCMPEQB Z3, Z12, K1, K3
	VPCMPEQB Z4, Z13, K1, K4
	KANDQ K3, K4, K3
	KORQ K5, K3, K5
	VPCMPEQB Z5, Z14, K1, K3
	VPCMPEQB Z6, Z15, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z7, Z14, K1, K4
	VPCMPEQB Z8, Z15, K1, K6
	KANDQ K4, K6, K4
	KORQ K3, K4, K3
	KANDQ K5, K3, K5
	KORTESTQ K2, K5
	JNE pairpairfounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairpairdouble64
pairpairfounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairpairstop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP pairpairdone64

pairpairloop64:
	CMPQ DX, $64
	JL pairpairdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z9, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z0, K1, K3
	VPCMPEQB Z4, Z9, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	VPCMPEQB Z5, Z10, K1, K3
	VPCMPEQB Z6, Z11, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z7, Z10, K1, K4
	VPCMPEQB Z8, Z11, K1, K5
	KANDQ K4, K5, K4
	KORQ K3, K4, K3
	KANDQ K2, K3, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairpairstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairpairloop64
pairpairstop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairpairdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairSecondSkip32 scans the two raw UTF-8 continuation-byte values once per
// block and leaves their preceding bytes to the plan transition.
TEXT ·pairSecondSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVBLZX 17(SI), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 21(SI), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
pairsecondloop32:
	CMPQ DX, $32
	JL pairseconddone32
	VMOVDQU (AX), Y0
	VPCMPEQB Y1, Y0, Y3
	VPCMPEQB Y2, Y0, Y4
	VPMOVMSKB Y3, CX
	VPMOVMSKB Y4, R8
	ORL R8, CX
	TESTL CX, CX
	JNZ pairsecondstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP pairsecondloop32
pairsecondstop32:
	BSFL CX, CX
	ADDQ CX, BX
pairseconddone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairSecondSkip64 is the AVX-512 BW form of pairSecondSkip32.
TEXT ·pairSecondSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 17(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 21(SI), R8
	VPBROADCASTB R8, K1, Z2
pairsecondloop64:
	CMPQ DX, $64
	JL pairseconddone64
	VMOVDQU8 (AX), K1, Z0
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z0, K1, K3
	KORQ K2, K3, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairsecondstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairsecondloop64
pairsecondstop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairseconddone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// filterSkip32 scans a rootFilter's one-byte candidates and UTF-8 prefixes.
// rootFilter's fixed layout is ones[16] at 0, pairs[16][4] at 16, then the
// one and pair counts at offsets 80 and 81.
TEXT ·filterSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $0x2020202020202020, R11
	VMOVQ R11, X8
	VPBROADCASTQ X8, Y8

filterloop32:
	CMPQ DX, $33
	JL filterdone32
	VMOVDQU (AX), Y0
	VMOVDQU 1(AX), Y5
	VPOR Y8, Y0, Y6
	VPOR Y8, Y5, Y7
	XORL CX, CX
	XORQ DI, DI
	MOVBLZX 80(SI), R10
filterones32:
	CMPQ DI, R10
	JGE filterpairsstart32
	MOVBLZX (SI)(DI*1), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	VPCMPEQB Y1, Y0, Y1
	VPMOVMSKB Y1, R8
	ORL R8, CX
	INCQ DI
	JMP filterones32
filterpairsstart32:
	XORQ DI, DI
	MOVBLZX 81(SI), R10
filterpairs32:
	CMPQ DI, R10
	JGE filtermask32
	MOVBLZX 16(SI)(DI*4), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 17(SI)(DI*4), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
	MOVBLZX 18(SI)(DI*4), R11
	TESTQ $1, R11
	JZ filterrawfirst32
	VPCMPEQB Y1, Y6, Y1
	JMP filterfirstdone32
filterrawfirst32:
	VPCMPEQB Y1, Y0, Y1
filterfirstdone32:
	TESTQ $2, R11
	JZ filterrawsecond32
	VPCMPEQB Y2, Y7, Y2
	JMP filterseconddone32
filterrawsecond32:
	VPCMPEQB Y2, Y5, Y2
filterseconddone32:
	VPMOVMSKB Y1, R8
	VPMOVMSKB Y2, R9
	ANDL R9, R8
	ORL R8, CX
	INCQ DI
	JMP filterpairs32
filtermask32:
	TESTL CX, CX
	JNZ filterstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP filterloop32
filterstop32:
	BSFL CX, CX
	ADDQ CX, BX
filterdone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// filterSkip64 is the AVX-512 BW version of filterSkip32.
TEXT ·filterSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ $0x2020202020202020, R11
	VPBROADCASTB R11, K1, Z8

filterloop64:
	CMPQ DX, $65
	JL filterdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VPORQ Z8, Z0, K1, Z6
	VPORQ Z8, Z5, K1, Z7
	MOVQ $0, CX
	KMOVQ CX, K2
	XORQ DI, DI
	MOVBLZX 80(SI), R10
filterones64:
	CMPQ DI, R10
	JGE filterpairsstart64
	MOVBLZX (SI)(DI*1), R8
	VPBROADCASTB R8, K1, Z1
	VPCMPEQB Z1, Z0, K1, K3
	KORQ K2, K3, K2
	INCQ DI
	JMP filterones64
filterpairsstart64:
	XORQ DI, DI
	MOVBLZX 81(SI), R10
filterpairs64:
	CMPQ DI, R10
	JGE filtermask64
	MOVBLZX 16(SI)(DI*4), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 17(SI)(DI*4), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 18(SI)(DI*4), R11
	TESTQ $1, R11
	JZ filterrawfirst64
	VPCMPEQB Z1, Z6, K1, K3
	JMP filterfirstdone64
filterrawfirst64:
	VPCMPEQB Z1, Z0, K1, K3
filterfirstdone64:
	TESTQ $2, R11
	JZ filterrawsecond64
	VPCMPEQB Z2, Z7, K1, K4
	JMP filterseconddone64
filterrawsecond64:
	VPCMPEQB Z2, Z5, K1, K4
filterseconddone64:
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	INCQ DI
	JMP filterpairs64
filtermask64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ filterstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP filterloop64
filterstop64:
	BSFQ CX, CX
	ADDQ CX, BX
filterdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// triplePairSkip32 specializes two fully ASCII-folded triples. Constants are
// broadcast once rather than once per filter entry per block.
TEXT ·triplePairSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVBLZX 0(SI), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 1(SI), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
	MOVBLZX 2(SI), R8
	VMOVQ R8, X3
	VPBROADCASTB X3, Y3
	MOVBLZX 4(SI), R8
	VMOVQ R8, X4
	VPBROADCASTB X4, Y4
	MOVBLZX 5(SI), R8
	VMOVQ R8, X5
	VPBROADCASTB X5, Y5
	MOVBLZX 6(SI), R8
	VMOVQ R8, X6
	VPBROADCASTB X6, Y6
	MOVQ $0x2020202020202020, R8
	VMOVQ R8, X8
	VPBROADCASTQ X8, Y8
triplepairloop32:
	CMPQ DX, $34
	JL triplepairdone32
	VMOVDQU (AX), Y0
	VMOVDQU 1(AX), Y9
	VMOVDQU 2(AX), Y10
	VPOR Y8, Y0, Y0
	VPOR Y8, Y9, Y9
	VPOR Y8, Y10, Y10
	VPCMPEQB Y1, Y0, Y11
	VPCMPEQB Y2, Y9, Y12
	VPCMPEQB Y3, Y10, Y13
	VPMOVMSKB Y11, CX
	VPMOVMSKB Y12, R8
	ANDL R8, CX
	VPMOVMSKB Y13, R8
	ANDL R8, CX
	VPCMPEQB Y4, Y0, Y11
	VPCMPEQB Y5, Y9, Y12
	VPCMPEQB Y6, Y10, Y13
	VPMOVMSKB Y11, R8
	VPMOVMSKB Y12, R9
	ANDL R9, R8
	VPMOVMSKB Y13, R9
	ANDL R9, R8
	ORL R8, CX
	TESTL CX, CX
	JNZ triplepairstop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP triplepairloop32
triplepairstop32:
	BSFL CX, CX
	ADDQ CX, BX
triplepairdone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// triplePairSkip64 is the AVX-512 BW form of triplePairSkip32.
TEXT ·triplePairSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 4(SI), R8
	VPBROADCASTB R8, K1, Z4
	MOVBLZX 5(SI), R8
	VPBROADCASTB R8, K1, Z5
	MOVBLZX 6(SI), R8
	VPBROADCASTB R8, K1, Z6
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8
triplepairloop64:
	CMPQ DX, $66
	JL triplepairdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z8, Z9, K1, Z9
	VPORQ Z8, Z10, K1, Z10
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z9, K1, K3
	VPCMPEQB Z3, Z10, K1, K4
	KANDQ K2, K3, K2
	KANDQ K2, K4, K2
	VPCMPEQB Z4, Z0, K1, K3
	VPCMPEQB Z5, Z9, K1, K4
	VPCMPEQB Z6, Z10, K1, K5
	KANDQ K3, K4, K3
	KANDQ K3, K5, K3
	KORQ K2, K3, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ triplepairstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP triplepairloop64
triplepairstop64:
	BSFQ CX, CX
	ADDQ CX, BX
triplepairdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleMixedSkip64 specializes two fully folded ASCII triples plus one raw
// UTF-8 triple whose third byte is an ASCII-folded continuation. It keeps the
// constants resident across blocks while preserving the complete OR-set.
TEXT ·tripleMixedSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 4(SI), R8
	VPBROADCASTB R8, K1, Z4
	MOVBLZX 5(SI), R8
	VPBROADCASTB R8, K1, Z5
	MOVBLZX 6(SI), R8
	VPBROADCASTB R8, K1, Z6
	MOVBLZX 8(SI), R8
	VPBROADCASTB R8, K1, Z9
	MOVBLZX 9(SI), R8
	VPBROADCASTB R8, K1, Z10
	MOVBLZX 10(SI), R8
	VPBROADCASTB R8, K1, Z11
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8
triplemixedloop64:
	CMPQ DX, $66
	JL triplemixeddone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z12
	VMOVDQU8 2(AX), K1, Z13
	VPORQ Z8, Z0, K1, Z14
	VPORQ Z8, Z12, K1, Z15
	VPORQ Z8, Z13, K1, Z16
	VPCMPEQB Z1, Z14, K1, K2
	VPCMPEQB Z2, Z15, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z16, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z4, Z14, K1, K3
	VPCMPEQB Z5, Z15, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z6, Z16, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z9, Z0, K1, K4
	VPCMPEQB Z10, Z12, K1, K5
	KANDQ K4, K5, K4
	VPCMPEQB Z11, Z16, K1, K5
	KANDQ K4, K5, K4
	KORQ K2, K3, K2
	KORQ K2, K4, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ triplemixedstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP triplemixedloop64
triplemixedstop64:
	BSFQ CX, CX
	ADDQ CX, BX
triplemixeddone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleASCIIUTF8Skip64 specializes one fully folded ASCII triple plus one
// raw UTF-8 triple. It is the two-orbit root shape for k/K/Kelvin-style
// literals; keeping the six broadcast constants live avoids rebuilding them
// inside tripleSkip64's generic per-value loop.
TEXT ·tripleASCIIUTF8Skip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 4(SI), R8
	VPBROADCASTB R8, K1, Z4
	MOVBLZX 5(SI), R8
	VPBROADCASTB R8, K1, Z5
	MOVBLZX 6(SI), R8
	VPBROADCASTB R8, K1, Z6
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8

tripleasciiutf8double64:
	CMPQ DX, $130
	JL tripleasciiutf8loop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10
	VMOVDQU8 64(AX), K1, Z11
	VMOVDQU8 65(AX), K1, Z12
	VMOVDQU8 66(AX), K1, Z13

	// Current ASCII rendering.
	VPORQ Z8, Z0, K1, Z14
	VPORQ Z8, Z9, K1, Z15
	VPORQ Z8, Z10, K1, Z16
	VPCMPEQB Z1, Z14, K1, K2
	VPCMPEQB Z2, Z15, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z16, K1, K3
	KANDQ K2, K3, K2
	// Current Kelvin rendering.
	VPCMPEQB Z4, Z0, K1, K3
	VPCMPEQB Z5, Z9, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z6, Z10, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2

	// Next block's raw rendering can be checked before reusing the vectors for
	// its folded ASCII rendering.
	VPCMPEQB Z4, Z11, K1, K4
	VPCMPEQB Z5, Z12, K1, K5
	KANDQ K4, K5, K4
	VPCMPEQB Z6, Z13, K1, K5
	KANDQ K4, K5, K4
	VPORQ Z8, Z11, K1, Z14
	VPORQ Z8, Z12, K1, Z15
	VPORQ Z8, Z13, K1, Z16
	VPCMPEQB Z1, Z14, K1, K5
	VPCMPEQB Z2, Z15, K1, K6
	KANDQ K5, K6, K5
	VPCMPEQB Z3, Z16, K1, K6
	KANDQ K5, K6, K5
	KORQ K5, K4, K4
	KORTESTQ K2, K4
	JNE tripleasciiutf8founddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP tripleasciiutf8double64
tripleasciiutf8founddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ tripleasciiutf8stop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP tripleasciiutf8done64

tripleasciiutf8loop64:
	CMPQ DX, $66
	JL tripleasciiutf8done64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10
	VPORQ Z8, Z0, K1, Z14
	VPORQ Z8, Z9, K1, Z15
	VPORQ Z8, Z10, K1, Z16
	VPCMPEQB Z1, Z14, K1, K2
	VPCMPEQB Z2, Z15, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z16, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z4, Z0, K1, K3
	VPCMPEQB Z5, Z9, K1, K4
	KANDQ K3, K4, K3
	VPCMPEQB Z6, Z10, K1, K4
	KANDQ K3, K4, K3
	KORQ K2, K3, K2
	KTESTQ K2, K2
	JNE tripleasciiutf8stop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP tripleasciiutf8loop64
tripleasciiutf8stop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
tripleasciiutf8done64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleSharedPrefixSkip64 specializes the ASCII/long-s orbit shape: two
// triples share their first two folded ASCII bytes, while their third byte is
// either folded ASCII or the first raw UTF-8 byte. The shared pair is tested
// once per lane before either third-byte rendering is admitted.
TEXT ·tripleSharedPrefixSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 6(SI), R8
	VPBROADCASTB R8, K1, Z4
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z8

tripleshareddouble64:
	CMPQ DX, $130
	JL triplesharedloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10
	VMOVDQU8 64(AX), K1, Z11
	VMOVDQU8 65(AX), K1, Z12
	VMOVDQU8 66(AX), K1, Z13
	VPORQ Z8, Z0, K1, Z14
	VPORQ Z8, Z9, K1, Z15
	VPORQ Z8, Z10, K1, Z16
	VPCMPEQB Z1, Z14, K1, K2
	VPCMPEQB Z2, Z15, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z16, K1, K3
	VPCMPEQB Z4, Z10, K1, K4
	KORQ K3, K4, K3
	KANDQ K2, K3, K2
	VPORQ Z8, Z11, K1, Z14
	VPORQ Z8, Z12, K1, Z15
	VPORQ Z8, Z13, K1, Z16
	VPCMPEQB Z1, Z14, K1, K4
	VPCMPEQB Z2, Z15, K1, K5
	KANDQ K4, K5, K4
	VPCMPEQB Z3, Z16, K1, K5
	VPCMPEQB Z4, Z13, K1, K6
	KORQ K5, K6, K5
	KANDQ K4, K5, K4
	KORTESTQ K2, K4
	JNE triplesharedfounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP tripleshareddouble64
triplesharedfounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ triplesharedstop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP tripleshareddone64

triplesharedloop64:
	CMPQ DX, $66
	JL tripleshareddone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10
	VPORQ Z8, Z0, K1, Z14
	VPORQ Z8, Z9, K1, Z15
	VPORQ Z8, Z10, K1, Z16
	VPCMPEQB Z1, Z14, K1, K2
	VPCMPEQB Z2, Z15, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z3, Z16, K1, K3
	VPCMPEQB Z4, Z10, K1, K4
	KORQ K3, K4, K3
	KANDQ K2, K3, K2
	KTESTQ K2, K2
	JNE triplesharedstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP triplesharedloop64
triplesharedstop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
tripleshareddone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleSkip32 scans triples whose individual bytes may use ASCII folding.
// tripleFilter stores sixteen four-byte triples at offset zero and its active
// count at offset 64; byte three is the per-position fold mask.
TEXT ·tripleSkip32(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $0x2020202020202020, R11
	VMOVQ R11, X8
	VPBROADCASTQ X8, Y8

tripleloop32:
	CMPQ DX, $34
	JL tripledone32
	VMOVDQU (AX), Y0
	VMOVDQU 1(AX), Y5
	VMOVDQU 2(AX), Y6
	VPOR Y8, Y0, Y9
	VPOR Y8, Y5, Y10
	VPOR Y8, Y6, Y11
	XORL CX, CX
	XORQ DI, DI
	MOVBLZX 64(SI), R10
triplevalues32:
	CMPQ DI, R10
	JGE triplemask32
	MOVBLZX (SI)(DI*4), R8
	VMOVQ R8, X1
	VPBROADCASTB X1, Y1
	MOVBLZX 1(SI)(DI*4), R8
	VMOVQ R8, X2
	VPBROADCASTB X2, Y2
	MOVBLZX 2(SI)(DI*4), R8
	VMOVQ R8, X3
	VPBROADCASTB X3, Y3
	MOVBLZX 3(SI)(DI*4), R11
	TESTQ $1, R11
	JZ triplerawfirst32
	VPCMPEQB Y1, Y9, Y1
	JMP triplefirstdone32
triplerawfirst32:
	VPCMPEQB Y1, Y0, Y1
triplefirstdone32:
	TESTQ $2, R11
	JZ triplerawsecond32
	VPCMPEQB Y2, Y10, Y2
	JMP tripleseconddone32
triplerawsecond32:
	VPCMPEQB Y2, Y5, Y2
tripleseconddone32:
	TESTQ $4, R11
	JZ triplerawthird32
	VPCMPEQB Y3, Y11, Y3
	JMP triplethirddone32
triplerawthird32:
	VPCMPEQB Y3, Y6, Y3
triplethirddone32:
	VPMOVMSKB Y1, R8
	VPMOVMSKB Y2, R9
	ANDL R9, R8
	VPMOVMSKB Y3, R9
	ANDL R9, R8
	ORL R8, CX
	INCQ DI
	JMP triplevalues32
triplemask32:
	TESTL CX, CX
	JNZ triplestop32
	ADDQ $32, AX
	ADDQ $32, BX
	SUBQ $32, DX
	JMP tripleloop32
triplestop32:
	BSFL CX, CX
	ADDQ CX, BX
tripledone32:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleSkip64 is the AVX-512 BW version of tripleSkip32.
TEXT ·tripleSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVQ $0x2020202020202020, R11
	VPBROADCASTB R11, K1, Z8

tripleloop64:
	CMPQ DX, $66
	JL tripledone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z5
	VMOVDQU8 2(AX), K1, Z6
	VPORQ Z8, Z0, K1, Z9
	VPORQ Z8, Z5, K1, Z10
	VPORQ Z8, Z6, K1, Z11
	MOVQ $0, CX
	KMOVQ CX, K2
	XORQ DI, DI
	MOVBLZX 64(SI), R10
triplevalues64:
	CMPQ DI, R10
	JGE triplemask64
	MOVBLZX (SI)(DI*4), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI)(DI*4), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI)(DI*4), R8
	VPBROADCASTB R8, K1, Z3
	MOVBLZX 3(SI)(DI*4), R11
	TESTQ $1, R11
	JZ triplerawfirst64
	VPCMPEQB Z1, Z9, K1, K3
	JMP triplefirstdone64
triplerawfirst64:
	VPCMPEQB Z1, Z0, K1, K3
triplefirstdone64:
	TESTQ $2, R11
	JZ triplerawsecond64
	VPCMPEQB Z2, Z10, K1, K4
	JMP tripleseconddone64
triplerawsecond64:
	VPCMPEQB Z2, Z5, K1, K4
tripleseconddone64:
	TESTQ $4, R11
	JZ triplerawthird64
	VPCMPEQB Z3, Z11, K1, K5
	JMP triplethirddone64
triplerawthird64:
	VPCMPEQB Z3, Z6, K1, K5
triplethirddone64:
	KANDQ K3, K4, K3
	KANDQ K3, K5, K3
	KORQ K2, K3, K2
	INCQ DI
	JMP triplevalues64
triplemask64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ triplestop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP tripleloop64
triplestop64:
	BSFQ CX, CX
	ADDQ CX, BX
tripledone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairShuftiSkip64 recognizes a bounded raw-pair union using two groups of
// nibble-to-slot tables. Each VPSHUFB looks up a byte's low or high nibble;
// intersecting all four lookups leaves a nonzero slot bit exactly when the
// adjacent source bytes form one of that group's expanded root pairs. The
// caller supplies enough tail slack for the overlapping second-byte load.
TEXT ·pairShuftiSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1

	// pairShuftiGroup is four consecutive 16-byte tables. Broadcast each one
	// into all four VPSHUFB lanes once, before scanning the haystack blocks.
	VBROADCASTI32X4 0(SI), K1, Z1
	VBROADCASTI32X4 16(SI), K1, Z2
	VBROADCASTI32X4 32(SI), K1, Z3
	VBROADCASTI32X4 48(SI), K1, Z4
	VBROADCASTI32X4 64(SI), K1, Z5
	VBROADCASTI32X4 80(SI), K1, Z6
	VBROADCASTI32X4 96(SI), K1, Z7
	VBROADCASTI32X4 112(SI), K1, Z8
	MOVQ $0x0f0f0f0f0f0f0f0f, R8
	VPBROADCASTB R8, K1, Z15

pairshuftiloop64:
	// The second load is one byte displaced, so a complete block needs 65
	// bytes remaining. The Go wrapper handles the final partial candidates.
	CMPQ DX, $65
	JL pairshuftidone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VPANDQ Z15, Z0, K1, Z10
	VPSRLW $4, Z0, K1, Z11
	VPANDQ Z15, Z11, K1, Z11
	VPANDQ Z15, Z9, K1, Z12
	VPSRLW $4, Z9, K1, Z13
	VPANDQ Z15, Z13, K1, Z13

	// Group zero: table lookups are written in Plan 9's control,table order.
	VPSHUFB Z10, Z1, K1, Z14
	VPSHUFB Z11, Z2, K1, Z16
	VPSHUFB Z12, Z3, K1, Z17
	// 0x80 selects the three-way AND; the fourth lookup stays independent.
	VPTERNLOGD $0x80, Z17, Z16, K1, Z14
	VPSHUFB Z13, Z4, K1, Z16
	VPANDQ Z16, Z14, K1, Z14
	VPTESTMB Z14, Z14, K1, K2

	// Group one holds the remaining raw pairs. A zero-filled unused group is
	// harmless, but the compiler enables this transition only for two groups.
	VPSHUFB Z10, Z5, K1, Z14
	VPSHUFB Z11, Z6, K1, Z16
	VPSHUFB Z12, Z7, K1, Z17
	VPTERNLOGD $0x80, Z17, Z16, K1, Z14
	VPSHUFB Z13, Z8, K1, Z16
	VPANDQ Z16, Z14, K1, Z14
	VPTESTMB Z14, Z14, K1, K3
	KORQ K2, K3, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairshuftistop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairshuftiloop64
pairshuftistop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairshuftidone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairShuftiWithOnesSkip64 is the dense mixed-root form. It checks two raw
// one-byte roots directly, then applies the nibble tables to byte-5-normalized
// pairs. Normalization may admit extra pair survivors but cannot skip a root;
// the decoded plan remains the semantic authority after every stop.
TEXT ·pairShuftiWithOnesSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1

	VBROADCASTI32X4 0(SI), K1, Z1
	VBROADCASTI32X4 16(SI), K1, Z2
	VBROADCASTI32X4 32(SI), K1, Z3
	VBROADCASTI32X4 48(SI), K1, Z4
	VBROADCASTI32X4 64(SI), K1, Z5
	VBROADCASTI32X4 80(SI), K1, Z6
	VBROADCASTI32X4 96(SI), K1, Z7
	VBROADCASTI32X4 112(SI), K1, Z8
	MOVBLZX 128(SI), R8
	VPBROADCASTB R8, K1, Z17
	MOVBLZX 129(SI), R8
	VPBROADCASTB R8, K1, Z18
	MOVQ $0x0f0f0f0f0f0f0f0f, R8
	VPBROADCASTB R8, K1, Z15
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z19

pairshuftionesloop64:
	CMPQ DX, $65
	JL pairshuftionesdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VPCMPEQB Z17, Z0, K1, K4
	VPCMPEQB Z18, Z0, K1, K5
	KORQ K4, K5, K4
	VPORQ Z19, Z0, K1, Z0
	VPORQ Z19, Z9, K1, Z9
	VPANDQ Z15, Z0, K1, Z10
	VPSRLW $4, Z0, K1, Z11
	VPANDQ Z15, Z11, K1, Z11
	VPANDQ Z15, Z9, K1, Z12
	VPSRLW $4, Z9, K1, Z13
	VPANDQ Z15, Z13, K1, Z13

	VPSHUFB Z10, Z1, K1, Z14
	VPSHUFB Z11, Z2, K1, Z16
	VPSHUFB Z12, Z3, K1, Z20
	VPTERNLOGD $0x80, Z20, Z16, K1, Z14
	VPSHUFB Z13, Z4, K1, Z16
	VPANDQ Z16, Z14, K1, Z14
	VPTESTMB Z14, Z14, K1, K2

	VPSHUFB Z10, Z5, K1, Z14
	VPSHUFB Z11, Z6, K1, Z16
	VPSHUFB Z12, Z7, K1, Z20
	VPTERNLOGD $0x80, Z20, Z16, K1, Z14
	VPSHUFB Z13, Z8, K1, Z16
	VPANDQ Z16, Z14, K1, Z14
	VPTESTMB Z14, Z14, K1, K3
	KORQ K2, K3, K2
	KORQ K4, K2, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairshuftionesstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairshuftionesloop64
pairshuftionesstop64:
	BSFQ CX, CX
	ADDQ CX, BX
pairshuftionesdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiPairDirectSkip64 is the large-input direct-load form of the byte-zero
// and byte-eight literal filter. Independent blocks expose enough work to hide
// load and compare latency without the carried short-input VALIGNQ transition.
TEXT ·asciiPairDirectSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z8
	MOVBLZX 3(SI), R8
	VPBROADCASTB R8, K1, Z9

asciipairdirectquad64:
	CMPQ DX, $256
	JL asciipairdirectdouble64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 8(AX), K1, Z3
	VMOVDQU8 64(AX), K1, Z4
	VMOVDQU8 72(AX), K1, Z5
	VMOVDQU8 128(AX), K1, Z6
	VMOVDQU8 136(AX), K1, Z7
	VMOVDQU8 192(AX), K1, Z10
	VMOVDQU8 200(AX), K1, Z11
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z9, Z3, K1, Z3
	VPORQ Z8, Z4, K1, Z4
	VPORQ Z9, Z5, K1, Z5
	VPORQ Z8, Z6, K1, Z6
	VPORQ Z9, Z7, K1, Z7
	VPORQ Z8, Z10, K1, Z10
	VPORQ Z9, Z11, K1, Z11
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z3, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z1, Z4, K1, K4
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K4, K3, K4
	VPCMPEQB Z1, Z6, K1, K5
	VPCMPEQB Z2, Z7, K1, K3
	KANDQ K5, K3, K5
	VPCMPEQB Z1, Z10, K1, K6
	VPCMPEQB Z2, Z11, K1, K3
	KANDQ K6, K3, K6
	KORTESTQ K2, K4
	JNE asciipairdirectfoundfirstquad64
	KORTESTQ K5, K6
	JNE asciipairdirectfoundsecondquad64
	ADDQ $256, AX
	ADDQ $256, BX
	SUBQ $256, DX
	JMP asciipairdirectquad64
asciipairdirectfoundfirstquad64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairdirectstop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciipairdirectdone64
asciipairdirectfoundsecondquad64:
	KMOVQ K5, CX
	TESTQ CX, CX
	JNZ asciipairdirectfoundthirdquad64
	KMOVQ K6, CX
	BSFQ CX, CX
	ADDQ $192, BX
	ADDQ CX, BX
	JMP asciipairdirectdone64
asciipairdirectfoundthirdquad64:
	BSFQ CX, CX
	ADDQ $128, BX
	ADDQ CX, BX
	JMP asciipairdirectdone64

asciipairdirectdouble64:
	CMPQ DX, $128
	JL asciipairdirectloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 8(AX), K1, Z3
	VMOVDQU8 64(AX), K1, Z4
	VMOVDQU8 72(AX), K1, Z5
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z9, Z3, K1, Z3
	VPORQ Z8, Z4, K1, Z4
	VPORQ Z9, Z5, K1, Z5
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z3, K1, K3
	KANDQ K2, K3, K2
	VPCMPEQB Z1, Z4, K1, K4
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K4, K3, K4
	KORTESTQ K2, K4
	JNE asciipairdirectfounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP asciipairdirectdouble64
asciipairdirectfounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairdirectstop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciipairdirectdone64

asciipairdirectloop64:
	CMPQ DX, $64
	JL asciipairdirectdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 8(AX), K1, Z3
	VPORQ Z8, Z0, K1, Z0
	VPORQ Z9, Z3, K1, Z3
	VPCMPEQB Z1, Z0, K1, K2
	VPCMPEQB Z2, Z3, K1, K3
	KANDQ K2, K3, K2
	KTESTQ K2, K2
	JNE asciipairdirectstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciipairdirectloop64
asciipairdirectstop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
asciipairdirectdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiPairShortSkip64 is the smaller two-block version for short haystacks.
// It avoids the larger steady-state setup when only a few blocks are scanned.
TEXT ·asciiPairShortSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1

	MOVBLZX 0(SI), R8
	VPBROADCASTB R8, K1, Z1
	MOVBLZX 1(SI), R8
	VPBROADCASTB R8, K1, Z2
	MOVBLZX 2(SI), R8
	VPBROADCASTB R8, K1, Z8
	MOVBLZX 3(SI), R8
	VPBROADCASTB R8, K1, Z9

	CMPQ DX, $128
	JL asciipairshorttail64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 64(AX), K1, Z3
	VMOVDQU8 128(AX), K1, Z6
asciipairshortloop128:
	// Current block: byte zero in Z0 and byte eight in Z0/Z3.
	VPORQ Z8, Z0, K1, Z4
	VALIGNQ $1, Z0, Z3, K1, Z5
	VPORQ Z9, Z5, K1, Z5
	VPCMPEQB Z1, Z4, K1, K2
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K2, K3, K2

	// Next block is independent after Z3/Z6 have been loaded, so evaluate it
	// before testing either mask. A candidate is rare; this avoids a per-block
	// branch and lets the two VALIGNQ transitions overlap.
	VPORQ Z8, Z3, K1, Z4
	VALIGNQ $1, Z3, Z6, K1, Z5
	VPORQ Z9, Z5, K1, Z5
	VPCMPEQB Z1, Z4, K1, K4
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K4, K3, K4
	// KORTEST sets ZF exactly when neither block has a candidate, avoiding a
	// k-to-GPR transfer on the overwhelmingly common miss path.
	KORTESTQ K2, K4
	JNE asciipairshortfound128
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	CMPQ DX, $128
	JL asciipairshorttail64
	VMOVDQA64 Z6, K1, Z0
	VMOVDQU8 64(AX), K1, Z3
	VMOVDQU8 128(AX), K1, Z6
	JMP asciipairshortloop128
asciipairshortfound128:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairshortstop64
	KMOVQ K4, CX
	BSFQ CX, CX
	ADDQ $64, CX
	ADDQ CX, BX
	JMP asciipairshortdone64

	// One final vector block needs only a single carried look-ahead load. The
	// wrapper's literal-width bound makes the byte-eight overlap in-bounds.
asciipairshorttail64:
	CMPQ DX, $64
	JL asciipairshortdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 64(AX), K1, Z3
	VPORQ Z8, Z0, K1, Z4
	VALIGNQ $1, Z0, Z3, K1, Z5
	VPORQ Z9, Z5, K1, Z5
	VPCMPEQB Z1, Z4, K1, K2
	VPCMPEQB Z2, Z5, K1, K3
	KANDQ K2, K3, K2
	KTESTQ K2, K2
	JNE asciipairshorttailstop64
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciipairshorttail64
asciipairshorttailstop64:
	KMOVQ K2, CX
asciipairshortstop64:
	BSFQ CX, CX
	ADDQ CX, BX
asciipairshortdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// tripleShuftiSkip64 scans a bounded union of three-byte forms. Six nibble
// tables assign every form a slot bit; a lane is a survivor only when all six
// lookups retain one common slot. The tables already encode exact ASCII cases,
// and the decoded plan transition verifies the surviving result.
TEXT ·tripleShuftiSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1

	// tripleShuftiFilter stores six consecutive 16-byte nibble tables.
	VBROADCASTI32X4 0(SI), K1, Z1
	VBROADCASTI32X4 16(SI), K1, Z2
	VBROADCASTI32X4 32(SI), K1, Z3
	VBROADCASTI32X4 48(SI), K1, Z4
	VBROADCASTI32X4 64(SI), K1, Z5
	VBROADCASTI32X4 80(SI), K1, Z6
	MOVQ $0x0f0f0f0f0f0f0f0f, R8
	VPBROADCASTB R8, K1, Z15

tripleshufloop64:
	// Three overlapping source vectors describe the 64 candidate starts. The
	// Go wrapper requires two trailing bytes before entering this loop.
	CMPQ DX, $66
	JL tripleshufdone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 2(AX), K1, Z10

	// Form table indexes: low and high nibbles of each byte position.
	VPANDQ Z15, Z0, K1, Z11
	VPSRLW $4, Z0, K1, Z12
	VPANDQ Z15, Z12, K1, Z12
	VPANDQ Z15, Z9, K1, Z13
	VPSRLW $4, Z9, K1, Z14
	VPANDQ Z15, Z14, K1, Z14
	VPANDQ Z15, Z10, K1, Z17
	VPSRLW $4, Z10, K1, Z18
	VPANDQ Z15, Z18, K1, Z18

	VPSHUFB Z11, Z1, K1, Z19
	VPSHUFB Z12, Z2, K1, Z20
	VPANDQ Z20, Z19, K1, Z19
	VPSHUFB Z13, Z3, K1, Z20
	VPANDQ Z20, Z19, K1, Z19
	VPSHUFB Z14, Z4, K1, Z20
	VPANDQ Z20, Z19, K1, Z19
	VPSHUFB Z17, Z5, K1, Z20
	VPANDQ Z20, Z19, K1, Z19
	VPSHUFB Z18, Z6, K1, Z20
	VPANDQ Z20, Z19, K1, Z19
	VPTESTMB Z19, Z19, K1, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ tripleshufstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP tripleshufloop64
tripleshufstop64:
	BSFQ CX, CX
	ADDQ CX, BX
tripleshufdone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiPairAnchorSkip64 scans one eight-slot pair projection. Four nibble
// lookups intersect the normalized source pair; any surviving lane is replayed
// by the Go caller through the shared decoded plan before it can be a match.
TEXT ·asciiPairAnchorSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1

	// asciiPairAnchorFilter stores four consecutive 16-byte nibble tables.
	VBROADCASTI32X4 0(SI), K1, Z1
	VBROADCASTI32X4 16(SI), K1, Z2
	VBROADCASTI32X4 32(SI), K1, Z3
	VBROADCASTI32X4 48(SI), K1, Z4
	MOVQ $0x0f0f0f0f0f0f0f0f, R8
	VPBROADCASTB R8, K1, Z15
	MOVQ $0x2020202020202020, R8
	VPBROADCASTB R8, K1, Z16

asciipairanchorloop64:
	// The second source vector is one byte displaced. The Go wrapper admits
	// this loop only with one trailing source byte beyond 64 candidates.
	CMPQ DX, $65
	JL asciipairanchordone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VPORQ Z16, Z0, K1, Z0
	VPORQ Z16, Z9, K1, Z9
	VPANDQ Z15, Z0, K1, Z10
	VPSRLW $4, Z0, K1, Z11
	VPANDQ Z15, Z11, K1, Z11
	VPANDQ Z15, Z9, K1, Z12
	VPSRLW $4, Z9, K1, Z13
	VPANDQ Z15, Z13, K1, Z13

	VPSHUFB Z10, Z1, K1, Z14
	VPSHUFB Z11, Z2, K1, Z17
	VPANDQ Z17, Z14, K1, Z14
	VPSHUFB Z12, Z3, K1, Z17
	VPANDQ Z17, Z14, K1, Z14
	VPSHUFB Z13, Z4, K1, Z17
	VPANDQ Z17, Z14, K1, Z14
	VPTESTMB Z14, Z14, K1, K2
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairanchorstop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciipairanchorloop64
asciipairanchorstop64:
	BSFQ CX, CX
	ADDQ CX, BX
asciipairanchordone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// probeVBMISkip64 is the AVX-512 VBMI form of probeSkip64. Its compiled
// low-six-bit tables preserve every true ASCII spelling and may over-admit a
// bit-six alias; the Go plan confirms each survivor before it can match.
TEXT ·probeVBMISkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	// asciiVBMIProbe stores its three byte offsets, then three 64-byte tables.
	MOVQ 0(SI), R8
	MOVQ 8(SI), R9
	MOVQ 16(SI), R10
	VMOVDQU8 24(SI), K1, Z1
	VMOVDQU8 88(SI), K1, Z2
	VMOVDQU8 152(SI), K1, Z3

// Four independent blocks amortize the loop control and expose the three
// VPERMB classification chains for each block before the first stop branch.
// Check the masks in source order below so the returned candidate remains the
// earliest table survivor.
probebvmiquad64:
	CMPQ DX, $256
	JL probebvmidouble64
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 (AX)(R9*1), K1, Z4
	VMOVDQU8 (AX)(R10*1), K1, Z5
	VMOVDQU8 64(AX)(R8*1), K1, Z9
	VMOVDQU8 64(AX)(R9*1), K1, Z10
	VMOVDQU8 64(AX)(R10*1), K1, Z11
	VMOVDQU8 128(AX)(R8*1), K1, Z12
	VMOVDQU8 128(AX)(R9*1), K1, Z13
	VMOVDQU8 128(AX)(R10*1), K1, Z14
	VMOVDQU8 192(AX)(R8*1), K1, Z15
	VMOVDQU8 192(AX)(R9*1), K1, Z16
	VMOVDQU8 192(AX)(R10*1), K1, Z17
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z4, Z4
	VPERMB Z3, Z5, Z5
	VPANDQ Z4, Z0, K1, Z0
	VPTESTMB Z5, Z0, K1, K2
	VPERMB Z1, Z9, Z9
	VPERMB Z2, Z10, Z10
	VPERMB Z3, Z11, Z11
	VPANDQ Z10, Z9, K1, Z9
	VPTESTMB Z11, Z9, K1, K3
	VPERMB Z1, Z12, Z12
	VPERMB Z2, Z13, Z13
	VPERMB Z3, Z14, Z14
	VPANDQ Z13, Z12, K1, Z12
	VPTESTMB Z14, Z12, K1, K4
	VPERMB Z1, Z15, Z15
	VPERMB Z2, Z16, Z16
	VPERMB Z3, Z17, Z17
	VPANDQ Z16, Z15, K1, Z15
	VPTESTMB Z17, Z15, K1, K5
	KORTESTQ K2, K3
	JNE probebvmifoundfirstquad64
	KORTESTQ K4, K5
	JNE probebvmifoundsecondquad64
	ADDQ $256, AX
	ADDQ $256, BX
	SUBQ $256, DX
	JMP probebvmiquad64
probebvmifoundfirstquad64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ probebvmistop64
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP probebvmidone64
probebvmifoundsecondquad64:
	KMOVQ K4, CX
	TESTQ CX, CX
	JNZ probebvmithirdstop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $192, BX
	ADDQ CX, BX
	JMP probebvmidone64
probebvmithirdstop64:
	BSFQ CX, CX
	ADDQ $128, BX
	ADDQ CX, BX
	JMP probebvmidone64

probebvmidouble64:
	CMPQ DX, $128
	JL probebvmiloop64
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 (AX)(R9*1), K1, Z4
	VMOVDQU8 (AX)(R10*1), K1, Z5
	VMOVDQU8 64(AX)(R8*1), K1, Z9
	VMOVDQU8 64(AX)(R9*1), K1, Z10
	VMOVDQU8 64(AX)(R10*1), K1, Z11
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z4, Z4
	VPERMB Z3, Z5, Z5
	VPANDQ Z4, Z0, K1, Z0
	VPTESTMB Z5, Z0, K1, K2
	VPERMB Z1, Z9, Z9
	VPERMB Z2, Z10, Z10
	VPERMB Z3, Z11, Z11
	VPANDQ Z10, Z9, K1, Z9
	VPTESTMB Z11, Z9, K1, K5
	KORTESTQ K2, K5
	JNE probebvmifounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP probebvmidouble64
probebvmifounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ probebvmistop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP probebvmidone64

probebvmiloop64:
	CMPQ DX, $64
	JL probebvmidone64
	VMOVDQU8 (AX)(R8*1), K1, Z0
	VMOVDQU8 (AX)(R9*1), K1, Z4
	VMOVDQU8 (AX)(R10*1), K1, Z5
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z4, Z4
	VPERMB Z3, Z5, Z5
	VPANDQ Z4, Z0, K1, Z0
	VPTESTMB Z5, Z0, K1, K2
	KTESTQ K2, K2
	JNE probebvmistop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP probebvmiloop64
probebvmistop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
probebvmidone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiPairDirectVBMISkip64 is the large-input direct-load form of the
// byte-zero and compiled-displacement literal filter. Two VPERMB tables
// replace per-byte folding and comparison; confirmation retains exact literal
// meaning.
TEXT ·asciiPairDirectVBMISkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ probe+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	VMOVDQU8 0(SI), K1, Z1
	VMOVDQU8 64(SI), K1, Z2
	MOVBLZX 128(SI), R8

asciipairdirectvbmiquad64:
	CMPQ DX, $256
	JL asciipairdirectvbmidouble64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 (AX)(R8*1), K1, Z3
	VMOVDQU8 64(AX), K1, Z4
	VMOVDQU8 64(AX)(R8*1), K1, Z5
	VMOVDQU8 128(AX), K1, Z6
	VMOVDQU8 128(AX)(R8*1), K1, Z7
	VMOVDQU8 192(AX), K1, Z10
	VMOVDQU8 192(AX)(R8*1), K1, Z11
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z3, Z3
	VPTESTMB Z3, Z0, K1, K2
	VPERMB Z1, Z4, Z4
	VPERMB Z2, Z5, Z5
	VPTESTMB Z5, Z4, K1, K3
	VPERMB Z1, Z6, Z6
	VPERMB Z2, Z7, Z7
	VPTESTMB Z7, Z6, K1, K4
	VPERMB Z1, Z10, Z10
	VPERMB Z2, Z11, Z11
	VPTESTMB Z11, Z10, K1, K5
	KORTESTQ K2, K3
	JNE asciipairdirectvbmifoundfirstquad64
	KORTESTQ K4, K5
	JNE asciipairdirectvbmifoundsecondquad64
	ADDQ $256, AX
	ADDQ $256, BX
	SUBQ $256, DX
	JMP asciipairdirectvbmiquad64
asciipairdirectvbmifoundfirstquad64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairdirectvbmifirststop64
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciipairdirectvbmidone64
asciipairdirectvbmifoundsecondquad64:
	KMOVQ K4, CX
	TESTQ CX, CX
	JNZ asciipairdirectvbmithirdstop64
	KMOVQ K5, CX
	BSFQ CX, CX
	ADDQ $192, BX
	ADDQ CX, BX
	JMP asciipairdirectvbmidone64
asciipairdirectvbmithirdstop64:
	BSFQ CX, CX
	ADDQ $128, BX
	ADDQ CX, BX
	JMP asciipairdirectvbmidone64

asciipairdirectvbmidouble64:
	CMPQ DX, $128
	JL asciipairdirectvbmiloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 (AX)(R8*1), K1, Z3
	VMOVDQU8 64(AX), K1, Z4
	VMOVDQU8 64(AX)(R8*1), K1, Z5
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z3, Z3
	VPTESTMB Z3, Z0, K1, K2
	VPERMB Z1, Z4, Z4
	VPERMB Z2, Z5, Z5
	VPTESTMB Z5, Z4, K1, K3
	KORTESTQ K2, K3
	JNE asciipairdirectvbmifounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP asciipairdirectvbmidouble64
asciipairdirectvbmifounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ asciipairdirectvbmifirststop64
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP asciipairdirectvbmidone64

asciipairdirectvbmiloop64:
	CMPQ DX, $64
	JL asciipairdirectvbmidone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 (AX)(R8*1), K1, Z3
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z3, Z3
	VPTESTMB Z3, Z0, K1, K2
	KTESTQ K2, K2
	JNE asciipairdirectvbmifirststop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciipairdirectvbmiloop64
asciipairdirectvbmifirststop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
asciipairdirectvbmidone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// asciiPairAnchorVBMISkip64 scans the compiled exact ASCII pair projection.
// VPERMT2B uses bit six to choose between each pair of 64-byte tables, so it
// replaces the four Shufti lookups without changing this filter's predicate.
TEXT ·asciiPairAnchorVBMISkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	VMOVDQU8 0(SI), K1, Z1
	VMOVDQU8 64(SI), K1, Z2
	VMOVDQU8 128(SI), K1, Z3
	VMOVDQU8 192(SI), K1, Z4

asciipairanchorvbmiloop64:
	CMPQ DX, $65
	JL asciipairanchorvbmidone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQA64 Z1, K1, Z10
	VPERMT2B Z2, Z0, Z10
	VMOVDQA64 Z3, K1, Z11
	VPERMT2B Z4, Z9, Z11
	VPTESTMB Z11, Z10, K1, K2
	KTESTQ K2, K2
	JNE asciipairanchorvbmistop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP asciipairanchorvbmiloop64
asciipairanchorvbmistop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
asciipairanchorvbmidone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairPairWordSkip64 compares adjacent raw-byte pairs as words. Separate even
// and odd source vectors retain every candidate start; BMI2 PDEP interleaves
// their rare-hit masks only after the block has passed the vector filter.
TEXT ·pairPairWordSkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	MOVWQZX 0(SI), R11
	VPBROADCASTW R11, K1, Z1
	MOVWQZX 2(SI), R11
	VPBROADCASTW R11, K1, Z2
	MOVWQZX 4(SI), R11
	VPBROADCASTW R11, K1, Z3
	MOVWQZX 6(SI), R11
	VPBROADCASTW R11, K1, Z4
	MOVBLZX 8(SI), R8

pairpairworddouble64:
	CMPQ DX, $128
	JL pairpairwordloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VMOVDQU8 64(AX), K1, Z12
	VMOVDQU8 65(AX), K1, Z13
	VMOVDQU8 64(AX)(R8*1), K1, Z14
	VMOVDQU8 65(AX)(R8*1), K1, Z15

	// First 64 starts: two primary values, then two confirmation values.
	VPCMPEQW Z1, Z0, K1, K2
	VPCMPEQW Z2, Z0, K1, K3
	KORQ K2, K3, K2
	VPCMPEQW Z1, Z9, K1, K3
	VPCMPEQW Z2, Z9, K1, K4
	KORQ K3, K4, K3
	VPCMPEQW Z3, Z10, K1, K4
	VPCMPEQW Z4, Z10, K1, K5
	KORQ K4, K5, K4
	KANDQ K2, K4, K2
	VPCMPEQW Z3, Z11, K1, K4
	VPCMPEQW Z4, Z11, K1, K5
	KORQ K4, K5, K4
	KANDQ K3, K4, K3

	// Second 64 starts are independent, so evaluate them before branching.
	VPCMPEQW Z1, Z12, K1, K4
	VPCMPEQW Z2, Z12, K1, K5
	KORQ K4, K5, K4
	VPCMPEQW Z1, Z13, K1, K5
	VPCMPEQW Z2, Z13, K1, K6
	KORQ K5, K6, K5
	VPCMPEQW Z3, Z14, K1, K6
	VPCMPEQW Z4, Z14, K1, K7
	KORQ K6, K7, K6
	KANDQ K4, K6, K4
	VPCMPEQW Z3, Z15, K1, K6
	VPCMPEQW Z4, Z15, K1, K7
	KORQ K6, K7, K6
	KANDQ K5, K6, K5
	KORTESTQ K2, K3
	JNE pairpairwordfoundfirstdouble64
	KORTESTQ K4, K5
	JNE pairpairwordfoundseconddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairpairworddouble64
pairpairwordfoundfirstdouble64:
	KMOVQ K2, CX
	KMOVQ K3, R11
	JMP pairpairwordposition64
pairpairwordfoundseconddouble64:
	KMOVQ K4, CX
	KMOVQ K5, R11
	ADDQ $64, BX
	JMP pairpairwordposition64

pairpairwordloop64:
	CMPQ DX, $64
	JL pairpairworddone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VPCMPEQW Z1, Z0, K1, K2
	VPCMPEQW Z2, Z0, K1, K3
	KORQ K2, K3, K2
	VPCMPEQW Z1, Z9, K1, K3
	VPCMPEQW Z2, Z9, K1, K4
	KORQ K3, K4, K3
	VPCMPEQW Z3, Z10, K1, K4
	VPCMPEQW Z4, Z10, K1, K5
	KORQ K4, K5, K4
	KANDQ K2, K4, K2
	VPCMPEQW Z3, Z11, K1, K4
	VPCMPEQW Z4, Z11, K1, K5
	KORQ K4, K5, K4
	KANDQ K3, K4, K3
	KORTESTQ K2, K3
	JNE pairpairwordfound64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairpairwordloop64
pairpairwordfound64:
	KMOVQ K2, CX
	KMOVQ K3, R11

pairpairwordposition64:
	// Expand word-lane masks into byte-start positions only after a hit.
	MOVQ $0x5555555555555555, R8
	PDEPQ R8, CX, CX
	MOVQ $0xaaaaaaaaaaaaaaaa, R8
	PDEPQ R8, R11, R11
	ORQ R11, CX
	BSFQ CX, CX
	ADDQ CX, BX
pairpairworddone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairPairVBMISkip64 intersects the two raw pair sets with four compiled
// VPERMB slot tables. The low-six-bit lookup is deliberately conservative;
// findUnicodePairAnchor confirms every stop through the same decoded plan.
TEXT ·pairPairVBMISkip64(SB), NOSPLIT, $0-32
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	VMOVDQU8 0(SI), K1, Z1
	VMOVDQU8 64(SI), K1, Z2
	VMOVDQU8 128(SI), K1, Z3
	VMOVDQU8 192(SI), K1, Z4
	MOVBLZX 256(SI), R8

pairpairvbmidouble64:
	CMPQ DX, $128
	JL pairpairvbmiloop64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VMOVDQU8 64(AX), K1, Z12
	VMOVDQU8 65(AX), K1, Z13
	VMOVDQU8 64(AX)(R8*1), K1, Z14
	VMOVDQU8 65(AX)(R8*1), K1, Z15

	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z3, Z10, Z10
	VPERMB Z4, Z11, Z11
	VPTESTMB Z9, Z0, K1, K2
	VPTESTMB Z11, Z10, K1, K3
	KANDQ K2, K3, K2

	VPERMB Z1, Z12, Z12
	VPERMB Z2, Z13, Z13
	VPERMB Z3, Z14, Z14
	VPERMB Z4, Z15, Z15
	VPTESTMB Z13, Z12, K1, K3
	VPTESTMB Z15, Z14, K1, K4
	KANDQ K3, K4, K3
	KORTESTQ K2, K3
	JNE pairpairvbmifounddouble64
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairpairvbmidouble64
pairpairvbmifounddouble64:
	KMOVQ K2, CX
	TESTQ CX, CX
	JNZ pairpairvbmistop64
	KMOVQ K3, CX
	BSFQ CX, CX
	ADDQ $64, BX
	ADDQ CX, BX
	JMP pairpairvbmidone64

pairpairvbmiloop64:
	CMPQ DX, $64
	JL pairpairvbmidone64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z3, Z10, Z10
	VPERMB Z4, Z11, Z11
	VPTESTMB Z9, Z0, K1, K2
	VPTESTMB Z11, Z10, K1, K3
	KANDQ K2, K3, K2
	KTESTQ K2, K2
	JNE pairpairvbmistop64
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairpairvbmiloop64
pairpairvbmistop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	ADDQ CX, BX
pairpairvbmidone64:
	MOVQ BX, ret+24(FP)
	VZEROUPPER
	RET

// pairPairConfirmVBMI64 keeps the pair-pair candidate mask in the AVX-512
// loop and checks each set bit against the bounded raw-token representation.
// The packed confirmation has ten-byte parts: values at 0, 2, and 4, source
// offset at 6, width at 7, and value count at 8. Its anchor offset and vector
// part count are at 201 and 202 after its twenty slots. The pair-pair slots
// are excluded from that count after their UTF-8 byte classes make the VBMI
// low-six-bit table hits exact.
TEXT ·pairPairConfirmVBMI64(SB), NOSPLIT, $0-48
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	MOVQ confirm+24(FP), DI
	XORQ BX, BX
	MOVQ $-1, CX
	KMOVQ CX, K1
	VMOVDQU8 0(SI), K1, Z1
	VMOVDQU8 64(SI), K1, Z2
	VMOVDQU8 128(SI), K1, Z3
	VMOVDQU8 192(SI), K1, Z4
	MOVBLZX 256(SI), R8

pairpairconfirmdouble64:
	CMPQ DX, $128
	JL pairpairconfirmsingle64
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VMOVDQU8 64(AX), K1, Z12
	VMOVDQU8 65(AX), K1, Z13
	VMOVDQU8 64(AX)(R8*1), K1, Z14
	VMOVDQU8 65(AX)(R8*1), K1, Z15

	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z3, Z10, Z10
	VPERMB Z4, Z11, Z11
	VPTESTMB Z9, Z0, K1, K2
	VPTESTMB Z11, Z10, K1, K3
	KANDQ K2, K3, K2

	VPERMB Z1, Z12, Z12
	VPERMB Z2, Z13, Z13
	VPERMB Z3, Z14, Z14
	VPERMB Z4, Z15, Z15
	VPTESTMB Z13, Z12, K1, K3
	VPTESTMB Z15, Z14, K1, K4
	KANDQ K3, K4, K3
	KORTESTQ K2, K3
	JEQ pairpairconfirmadvance128

	KMOVQ K2, CX
	XORQ SI, SI
	TESTQ CX, CX
	JNZ pairpairconfirmcandidate
	JMP pairpairconfirmsecond

pairpairconfirmsecond:
	MOVQ $1, SI
	KMOVQ K3, CX
	TESTQ CX, CX
	JNZ pairpairconfirmcandidate
	JMP pairpairconfirmadvance128

pairpairconfirmcandidate:
	BSFQ CX, R9
	LEAQ (AX)(R9*1), R10
	CMPQ SI, $1
	JNE pairpairconfirmbase
	ADDQ $64, R10
pairpairconfirmbase:
	MOVBLZX 201(DI), R13
	SUBQ R13, R10
	TESTB $2, 203(DI)
	JNE pairpairconfirmvariable
	LEAQ (R10)(R13*1), R11
	MOVQ $0x80C0, R14
	MOVWQZX (R11), R12
	ANDQ $0xC0C0, R12
	CMPQ R14, R12
	JNE pairpairconfirmreject
	MOVWQZX (R11)(R8*1), R12
	ANDQ $0xC0C0, R12
	CMPQ R14, R12
	JNE pairpairconfirmreject
	MOVQ DI, R11
	MOVBLZX 202(DI), R12
	TESTQ R12, R12
	JZ pairpairconfirmaccepted
pairpairconfirmpart:
	MOVBLZX 6(R11), R13
	MOVBLZX 7(R11), R14
	CMPQ R14, $2
	JEQ pairpairconfirmword
	MOVBLZX (R10)(R13*1), R14
	JMP pairpairconfirmvalue
pairpairconfirmword:
	MOVWQZX (R10)(R13*1), R14
pairpairconfirmvalue:
	MOVWQZX 0(R11), R15
	CMPQ R14, R15
	JEQ pairpairconfirmnext
	CMPB 8(R11), $2
	JL pairpairconfirmreject
	MOVWQZX 2(R11), R15
	CMPQ R14, R15
	JEQ pairpairconfirmnext
	CMPB 8(R11), $3
	JNE pairpairconfirmreject
	MOVWQZX 4(R11), R15
	CMPQ R14, R15
	JNE pairpairconfirmreject
pairpairconfirmnext:
	ADDQ $10, R11
	DECQ R12
	JNZ pairpairconfirmpart
	JMP pairpairconfirmaccepted

// Variable-width confirmation walks the raw forms in source order. Each
// ten-byte part contains up to three zero-padded three-byte forms and a count
// at byte nine. A form's UTF-8 lead byte determines whether the cursor advances
// by one, two, or three bytes.
pairpairconfirmvariable:
	MOVQ R10, R13
	MOVQ DI, R11
	MOVBLZX 202(DI), R12
pairpairconfirmvariablepart:
	MOVBLZX 9(R11), R14
	MOVQ R11, R15
pairpairconfirmvariableform:
	MOVBLZX 0(R15), R10
	CMPB 0(R13), R10
	JNE pairpairconfirmvariablenextform
	CMPQ R10, $0x80
	JL pairpairconfirmvariableone
	MOVBLZX 1(R15), R10
	CMPB 1(R13), R10
	JNE pairpairconfirmvariablenextform
	MOVBLZX 0(R15), R10
	CMPQ R10, $0xe0
	JL pairpairconfirmvariabletwo
	MOVBLZX 2(R15), R10
	CMPB 2(R13), R10
	JNE pairpairconfirmvariablenextform
	ADDQ $3, R13
	JMP pairpairconfirmvariablenextpart
pairpairconfirmvariabletwo:
	ADDQ $2, R13
	JMP pairpairconfirmvariablenextpart
pairpairconfirmvariableone:
	INCQ R13
pairpairconfirmvariablenextpart:
	ADDQ $10, R11
	DECQ R12
	JNZ pairpairconfirmvariablepart
	LEAQ (AX)(R9*1), R10
	CMPQ SI, $1
	JNE pairpairconfirmvariablewidth
	ADDQ $64, R10
pairpairconfirmvariablewidth:
	MOVBLZX 201(DI), R12
	SUBQ R12, R10
	SUBQ R10, R13
	JMP pairpairconfirmreturnaccepted
pairpairconfirmvariablenextform:
	ADDQ $3, R15
	DECQ R14
	JNZ pairpairconfirmvariableform
	JMP pairpairconfirmreject
pairpairconfirmaccepted:
	MOVBLZX 200(DI), R13
pairpairconfirmreturnaccepted:
	ADDQ R9, BX
	CMPQ SI, $1
	JNE pairpairconfirmdone
	ADDQ $64, BX
	JMP pairpairconfirmdone

pairpairconfirmreject:
	BTRQ R9, CX
	TESTQ CX, CX
	JNZ pairpairconfirmcandidate
	CMPQ SI, $0
	JEQ pairpairconfirmsecond
	CMPQ SI, $1
	JEQ pairpairconfirmadvance128
	JMP pairpairconfirmadvance64

pairpairconfirmadvance128:
	ADDQ $128, AX
	ADDQ $128, BX
	SUBQ $128, DX
	JMP pairpairconfirmdouble64

pairpairconfirmsingle64:
	CMPQ DX, $64
	JL pairpairconfirmdone
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 (AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z3, Z10, Z10
	VPERMB Z4, Z11, Z11
	VPTESTMB Z9, Z0, K1, K2
	VPTESTMB Z11, Z10, K1, K3
	KANDQ K2, K3, K2
	KTESTQ K2, K2
	JEQ pairpairconfirmadvance64
	KMOVQ K2, CX
	MOVQ $2, SI
	JMP pairpairconfirmcandidate

pairpairconfirmadvance64:
	ADDQ $64, AX
	ADDQ $64, BX
	SUBQ $64, DX
	JMP pairpairconfirmsingle64

pairpairconfirmdone:
	MOVQ n+8(FP), R12
	CMPQ BX, R12
	JNE pairpairconfirmwidthdone
	XORQ R13, R13
pairpairconfirmwidthdone:
	MOVQ BX, ret+32(FP)
	MOVQ R13, width+40(FP)
	VZEROUPPER
	RET

// rawByteMultiAnchorSkip64 intersects a primary pattern-tag pair with up to
// three fixed-displacement confirmation pairs. Tables use VPERMB's low six
// source bits. It returns the surviving tag byte with the first lane; Go then
// checks exact primary, confirmation, and guard pairs only for those tags.
// rawByteMultiAnchorFilter lays out primary tables at 0/64, confirmation-first
// tables at 128/192/256, and confirmation-second tables at 320/384/448.
TEXT ·rawByteMultiAnchorSkip64(SB), NOSPLIT, $64-33
	MOVQ ptr+0(FP), AX
	MOVQ n+8(FP), DX
	MOVQ filter+16(FP), SI
	MOVQ $-1, CX
	XORL R15, R15
	KMOVQ CX, K1
	VMOVDQU8 0(SI), K1, Z1
	VMOVDQU8 64(SI), K1, Z2
	VMOVDQU8 128(SI), K1, Z3
	VMOVDQU8 192(SI), K1, Z4
	VMOVDQU8 256(SI), K1, Z5
	VMOVDQU8 320(SI), K1, Z6
	VMOVDQU8 384(SI), K1, Z7
	VMOVDQU8 448(SI), K1, Z8
	MOVBLZX 512(SI), R8
	MOVBLZX 513(SI), R9
	MOVBLZX 514(SI), R10
	MOVBLZX 516(SI), R11
	ADDQ $65, R11
	LEAQ 192(R11), R12
	// The eight-block zero scan reads the adjacent primary byte through offset
	// 512. Keep a complete confirmation horizon for every scanned block too:
	// an aggregate hit must safely replay the four-/one-block dispatcher.
	LEAQ 448(R11), BX
rawbytemultianchorloop64:
	CMPQ DX, R11
	JL rawbytemultianchordone64
	CMPQ DX, BX
	JGE rawbytemultianchorzero512
rawbytemultianchorfour64:
	CMPQ DX, R12
	JL rawbytemultianchorsingle64

	// Four independent primary blocks hide VPERMB latency on no-candidate text.
	// Materialize their tag products, then reduce them once. The common sparse
	// no-hit path crosses only one byte mask; the per-block masks are derived
	// only when the aggregate is nonzero and are then reused for density choice.
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 64(AX), K1, Z10
	VMOVDQU8 65(AX), K1, Z11
	VMOVDQU8 128(AX), K1, Z12
	VMOVDQU8 129(AX), K1, Z13
	VMOVDQU8 192(AX), K1, Z14
	VMOVDQU8 193(AX), K1, Z15
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z1, Z10, Z10
	VPERMB Z2, Z11, Z11
	VPERMB Z1, Z12, Z12
	VPERMB Z2, Z13, Z13
	VPERMB Z1, Z14, Z14
	VPERMB Z2, Z15, Z15
	VPANDQ Z9, Z0, K1, Z0
	VPANDQ Z11, Z10, K1, Z10
	VPANDQ Z13, Z12, K1, Z12
	VPANDQ Z15, Z14, K1, Z14
	VPORQ Z10, Z0, K1, Z9
	// 0xfe is the three-input OR truth table. Z9 already carries blocks 0|1.
	VPTERNLOGD $0xfe, Z14, Z12, K1, Z9
	VPTESTMB Z9, Z9, K1, K2
	KTESTQ K2, K2
	JEQ rawbytemultianchoradvance256

	// An aggregate hit is rare on sparse data. Only then recover the individual
	// masks needed to select a sole block or enter the dense schedule.
	VPTESTMB Z0, Z0, K1, K2
	VPTESTMB Z10, Z10, K1, K3
	VPTESTMB Z12, Z12, K1, K4
	VPTESTMB Z14, Z14, K1, K5
	// Any two occupied blocks choose the bounded dense one-block schedule; a
	// sole occupied block keeps its already materialized primary product.
	KORTESTQ K2, K3
	JNE rawbytemultianchorfirstnonzero64
	KORTESTQ K4, K5
	JNE rawbytemultianchorlastnonzero64
rawbytemultianchoradvance256:
	ADDQ $256, AX
	SUBQ $256, DX
	JMP rawbytemultianchorloop64
rawbytemultianchorfirstnonzero64:
	KORTESTQ K4, K5
	JNE rawbytemultianchordense64
	KTESTQ K2, K2
	JEQ rawbytemultianchorblock164
	KTESTQ K3, K3
	JNE rawbytemultianchordense64
	JMP rawbytemultianchorconfirm64
rawbytemultianchorlastnonzero64:
	KTESTQ K4, K4
	JEQ rawbytemultianchorblock364
	KTESTQ K5, K5
	JNE rawbytemultianchordense64
rawbytemultianchorblock264:
	VMOVDQA64 Z12, K1, Z0
	ADDQ $128, AX
	SUBQ $128, DX
	JMP rawbytemultianchorconfirm64
rawbytemultianchorblock164:
	VMOVDQA64 Z10, K1, Z0
	ADDQ $64, AX
	SUBQ $64, DX
	JMP rawbytemultianchorconfirm64
rawbytemultianchorblock364:
	VMOVDQA64 Z14, K1, Z0
	ADDQ $192, AX
	SUBQ $192, DX
	JMP rawbytemultianchorconfirm64

// The sparse fast path has no candidate ordering work: it only proves that
// eight independent 64-byte primary blocks are all zero. Any aggregate hit
// replays the existing four-block dispatcher at the unchanged AX/DX, where it
// recovers masks, preserves leftmost order, and applies the density switch.
rawbytemultianchorzero512:
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 64(AX), K1, Z10
	VMOVDQU8 65(AX), K1, Z11
	VMOVDQU8 128(AX), K1, Z12
	VMOVDQU8 129(AX), K1, Z13
	VMOVDQU8 192(AX), K1, Z14
	VMOVDQU8 193(AX), K1, Z15
	VMOVDQU8 256(AX), K1, Z16
	VMOVDQU8 257(AX), K1, Z17
	VMOVDQU8 320(AX), K1, Z18
	VMOVDQU8 321(AX), K1, Z19
	VMOVDQU8 384(AX), K1, Z20
	VMOVDQU8 385(AX), K1, Z21
	VMOVDQU8 448(AX), K1, Z22
	VMOVDQU8 449(AX), K1, Z23
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z1, Z10, Z10
	VPERMB Z2, Z11, Z11
	VPERMB Z1, Z12, Z12
	VPERMB Z2, Z13, Z13
	VPERMB Z1, Z14, Z14
	VPERMB Z2, Z15, Z15
	VPERMB Z1, Z16, Z16
	VPERMB Z2, Z17, Z17
	VPERMB Z1, Z18, Z18
	VPERMB Z2, Z19, Z19
	VPERMB Z1, Z20, Z20
	VPERMB Z2, Z21, Z21
	VPERMB Z1, Z22, Z22
	VPERMB Z2, Z23, Z23
	VPANDQ Z9, Z0, K1, Z0
	VPANDQ Z11, Z10, K1, Z10
	VPANDQ Z13, Z12, K1, Z12
	VPANDQ Z15, Z14, K1, Z14
	VPANDQ Z17, Z16, K1, Z16
	VPANDQ Z19, Z18, K1, Z18
	VPANDQ Z21, Z20, K1, Z20
	VPANDQ Z23, Z22, K1, Z22
	// Reduce eight products in four Boolean operations. Each 0xfe ternary
	// instruction ORs its two sources with its old destination.
	VPTERNLOGD $0xfe, Z12, Z10, K1, Z0
	VPTERNLOGD $0xfe, Z18, Z16, K1, Z14
	VPTERNLOGD $0xfe, Z20, Z14, K1, Z0
	VPORQ Z22, Z0, K1, Z0
	VPTESTMB Z0, Z0, K1, K2
	KTESTQ K2, K2
	JNE rawbytemultianchorfour64
	ADDQ $512, AX
	SUBQ $512, DX
	JMP rawbytemultianchorloop64

rawbytemultianchorsingle64:
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPTESTMB Z9, Z0, K1, K2
	KTESTQ K2, K2
	JEQ rawbytemultianchoradvance64
	VPANDQ Z9, Z0, K1, Z0

rawbytemultianchorconfirm64:
	// Do not pay the three confirmation lookups for a block with no primary
	// tag. A primary hit remains conservative; the exact Go checks still decide
	// the candidate before the common raw transition replay.
	VMOVDQU8 0(AX)(R8*1), K1, Z9
	VMOVDQU8 1(AX)(R8*1), K1, Z10
	VMOVDQU8 0(AX)(R9*1), K1, Z11
	VMOVDQU8 1(AX)(R9*1), K1, Z12
	VMOVDQU8 0(AX)(R10*1), K1, Z13
	VMOVDQU8 1(AX)(R10*1), K1, Z14
	VPERMB Z3, Z9, Z9
	VPERMB Z6, Z10, Z10
	VPERMB Z4, Z11, Z11
	VPERMB Z7, Z12, Z12
	VPERMB Z5, Z13, Z13
	VPERMB Z8, Z14, Z14
	VPANDQ Z10, Z9, K1, Z9
	VPANDQ Z12, Z11, K1, Z11
	VPANDQ Z14, Z13, K1, Z13
	VPANDQ Z9, Z0, K1, Z9
	VPANDQ Z11, Z0, K1, Z11
	VPANDQ Z13, Z0, K1, Z13
	VPORQ Z11, Z9, K1, Z9
	VPORQ Z13, Z9, K1, Z9
	VPTESTMB Z9, Z9, K1, K2
	KTESTQ K2, K2
	JNE rawbytemultianchorstop64
rawbytemultianchoradvance64:
	ADDQ $64, AX
	SUBQ $64, DX
	JMP rawbytemultianchorloop64
rawbytemultianchorstop64:
	KMOVQ K2, CX
	BSFQ CX, CX
	VMOVDQU8 Z9, (SP)
	MOVBLZX (SP)(CX*1), R15
	ADDQ CX, AX
	JMP rawbytemultianchordone64
// The dense body is the proven one-block primary-plus-confirmation schedule.
// It changes only batching; the same conservative tables and Go replay remain
// the match authority.
rawbytemultianchordense64:
	// A dense-prefix/sparse-suffix diagnostic is three times slower when dense
	// mode owns the whole input. Bound a dense epoch to 4 KiB, or to the
	// remaining safe block count. The decrement below replaces the old per-block
	// tail comparison; expiration re-enters the shared four-block dispatcher.
	CMPQ DX, R11
	JL rawbytemultianchordone64
	MOVQ DX, R13
	SUBQ R11, R13
	SHRQ $6, R13
	INCQ R13
	CMPQ R13, $64
	JLE rawbytemultianchordenseloop64
	MOVQ $64, R13
rawbytemultianchordenseloop64:
	VMOVDQU8 (AX), K1, Z0
	VMOVDQU8 1(AX), K1, Z9
	VMOVDQU8 0(AX)(R8*1), K1, Z10
	VMOVDQU8 1(AX)(R8*1), K1, Z11
	VMOVDQU8 0(AX)(R9*1), K1, Z12
	VMOVDQU8 1(AX)(R9*1), K1, Z13
	VMOVDQU8 0(AX)(R10*1), K1, Z14
	VMOVDQU8 1(AX)(R10*1), K1, Z15
	VPERMB Z1, Z0, Z0
	VPERMB Z2, Z9, Z9
	VPERMB Z3, Z10, Z10
	VPERMB Z6, Z11, Z11
	VPERMB Z4, Z12, Z12
	VPERMB Z7, Z13, Z13
	VPERMB Z5, Z14, Z14
	VPERMB Z8, Z15, Z15
	VPANDQ Z9, Z0, K1, Z0
	VPANDQ Z11, Z10, K1, Z10
	VPANDQ Z13, Z12, K1, Z12
	VPANDQ Z15, Z14, K1, Z14
	VPANDQ Z10, Z0, K1, Z10
	VPANDQ Z12, Z0, K1, Z12
	VPANDQ Z14, Z0, K1, Z14
	VPORQ Z12, Z10, K1, Z10
	VPORQ Z14, Z10, K1, Z10
	VPTESTMB Z10, Z10, K1, K2
	KTESTQ K2, K2
	JNE rawbytemultianchordensestop64
	ADDQ $64, AX
	SUBQ $64, DX
	DECQ R13
	JNE rawbytemultianchordenseloop64
	JMP rawbytemultianchorloop64
rawbytemultianchordensestop64:
	VMOVDQA64 Z10, K1, Z9
	JMP rawbytemultianchorstop64

rawbytemultianchordone64:
	SUBQ ptr+0(FP), AX
	MOVQ AX, ret+24(FP)
	MOVB R15, tags+32(FP)
	VZEROUPPER
	RET
