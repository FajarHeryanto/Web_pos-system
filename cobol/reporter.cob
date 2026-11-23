       >>SOURCE FORMAT IS FREE
*> =====================================================================
*> reporter.cob - POS Batch Reporting Engine (GnuCOBOL 3.x, free format)
*>
*> Membaca processed_data.csv (hasil C Engine) dan menghasilkan laporan:
*>   A. Rekapitulasi Pendapatan Per Kategori + Grand Total
*>   B. Laporan Penjualan Harian
*>   C. Laporan Produk Terlaris Top 10
*>
*> Path file (urutan prioritas):
*>   1. Argumen CLI      : reporter <input.csv> <output.txt>
*>   2. Environment var  : INPUT_CSV_PATH dan OUTPUT_REPORT_PATH
*>   3. Default          : data/processed_data.csv, data/report_output.txt
*>
*> Return code: 0 sukses | 1 file input tidak ditemukan
*>              2 format tidak valid | 4 gagal menulis output
*>
*> Kompilasi  : cobc -x -free -o reporter reporter.cob
*> =====================================================================
IDENTIFICATION DIVISION.
PROGRAM-ID. REPORTER.

ENVIRONMENT DIVISION.
INPUT-OUTPUT SECTION.
FILE-CONTROL.
    SELECT IN-FILE ASSIGN TO DYNAMIC WS-IN-PATH
        ORGANIZATION IS LINE SEQUENTIAL
        FILE STATUS IS WS-IN-STATUS.
    SELECT OUT-FILE ASSIGN TO DYNAMIC WS-OUT-PATH
        ORGANIZATION IS LINE SEQUENTIAL
        FILE STATUS IS WS-OUT-STATUS.

DATA DIVISION.
FILE SECTION.
FD  IN-FILE.
01  IN-REC                      PIC X(512).

FD  OUT-FILE.
01  OUT-REC                     PIC X(100).

WORKING-STORAGE SECTION.
*> ---- Path & status file -------------------------------------------
01  WS-IN-PATH                  PIC X(256) VALUE SPACES.
01  WS-OUT-PATH                 PIC X(256) VALUE SPACES.
01  WS-CMD-LINE                 PIC X(520) VALUE SPACES.
01  WS-ARG-IN                   PIC X(256) VALUE SPACES.
01  WS-ARG-OUT                  PIC X(256) VALUE SPACES.
01  WS-IN-STATUS                PIC XX     VALUE "00".
01  WS-OUT-STATUS               PIC XX     VALUE "00".
01  WS-EOF-FLAG                 PIC X      VALUE "N".
    88  END-OF-FILE                        VALUE "Y".
01  WS-LINE-NO                  PIC 9(7)   VALUE 0.
01  WS-LINE-NO-ED               PIC Z(6)9.

*> ---- Field hasil UNSTRING -----------------------------------------
01  WS-FIELDS.
    05  F-ID                    PIC X(20).
    05  F-NAME                  PIC X(100).
    05  F-CAT                   PIC X(50).
    05  F-QTY                   PIC X(20).
    05  F-SUB                   PIC X(20).
    05  F-DISC                  PIC X(20).
    05  F-NET                   PIC X(20).
    05  F-PAY                   PIC X(20).
    05  F-DATE                  PIC X(20).
01  WS-FLD-COUNT                PIC 9(2)   VALUE 0.

*> ---- Nilai numerik record aktif -----------------------------------
01  N-QTY                       PIC 9(9)       VALUE 0.
01  N-SUB                       PIC S9(13)V99  VALUE 0.
01  N-DISC                      PIC S9(13)V99  VALUE 0.
01  N-NET                       PIC S9(13)V99  VALUE 0.

*> ---- Kunci pencarian & indeks -------------------------------------
01  WS-KEY-CAT                  PIC X(50)  VALUE SPACES.
01  WS-KEY-DATE                 PIC X(10)  VALUE SPACES.
01  WS-KEY-NAME                 PIC X(100)  VALUE SPACES.
01  WS-I                        PIC S9(4) COMP VALUE 0.
01  WS-J                        PIC S9(4) COMP VALUE 0.
01  WS-LIMIT                    PIC S9(4) COMP VALUE 0.
01  WS-FOUND-IDX                PIC S9(4) COMP VALUE 0.
01  WS-SKIPPED                  PIC 9(7)   VALUE 0.

*> ---- Grand total --------------------------------------------------
01  WS-REC-COUNT                PIC 9(7)       VALUE 0.
01  WS-TOT-QTY                  PIC 9(9)       VALUE 0.
01  WS-TOT-SUB                  PIC S9(13)V99  VALUE 0.
01  WS-TOT-DISC                 PIC S9(13)V99  VALUE 0.
01  WS-TOT-NET                  PIC S9(13)V99  VALUE 0.
01  WS-AVG-NET                  PIC S9(13)V99  VALUE 0.
01  WS-MIN-DATE                 PIC X(10)  VALUE HIGH-VALUES.
01  WS-MAX-DATE                 PIC X(10)  VALUE SPACES.

*> ---- Tabel kategori -----------------------------------------------
01  WS-CAT-COUNT                PIC S9(4) COMP VALUE 0.
01  CAT-TABLE.
    05  CAT-ENTRY OCCURS 50 TIMES.
        10  CAT-NAME            PIC X(50).
        10  CAT-TRX             PIC 9(7).
        10  CAT-QTY             PIC 9(9).
        10  CAT-SUB             PIC S9(13)V99.
        10  CAT-DISC            PIC S9(13)V99.
        10  CAT-NET             PIC S9(13)V99.

*> ---- Tabel penjualan harian ---------------------------------------
01  WS-DLY-COUNT                PIC S9(4) COMP VALUE 0.
01  DLY-TABLE.
    05  DLY-ENTRY OCCURS 400 TIMES.
        10  DLY-DATE            PIC X(10).
        10  DLY-TRX             PIC 9(7).
        10  DLY-QTY             PIC 9(9).
        10  DLY-NET             PIC S9(13)V99.
01  DLY-SWAP.
    05  DLY-SWAP-DATE           PIC X(10).
    05  DLY-SWAP-TRX            PIC 9(7).
    05  DLY-SWAP-QTY            PIC 9(9).
    05  DLY-SWAP-NET            PIC S9(13)V99.

*> ---- Tabel produk -------------------------------------------------
01  WS-PRD-COUNT                PIC S9(4) COMP VALUE 0.
01  PRD-TABLE.
    05  PRD-ENTRY OCCURS 500 TIMES.
        10  PRD-NAME            PIC X(100).
        10  PRD-CAT             PIC X(50).
        10  PRD-QTY             PIC 9(9).
        10  PRD-NET             PIC S9(13)V99.
01  PRD-SWAP.
    05  PRD-SWAP-NAME           PIC X(100).
    05  PRD-SWAP-CAT            PIC X(50).
    05  PRD-SWAP-QTY            PIC 9(9).
    05  PRD-SWAP-NET            PIC S9(13)V99.

*> ---- Tanggal & jam cetak ------------------------------------------
01  WS-NOW.
    05  WS-NOW-YYYY             PIC 9(4).
    05  WS-NOW-MM               PIC 9(2).
    05  WS-NOW-DD               PIC 9(2).
    05  WS-NOW-HH               PIC 9(2).
    05  WS-NOW-MI               PIC 9(2).
    05  WS-NOW-SS               PIC 9(2).
    05  FILLER                  PIC X(7).

*> ---- Baris cetak (lebar laporan 91 kolom) -------------------------
01  WS-SEP-EQ                   PIC X(91)  VALUE ALL "=".
01  WS-SEP-DASH                 PIC X(91)  VALUE ALL "-".
01  WS-COUNT-ED                 PIC Z,ZZZ,ZZ9.
01  WS-MONEY-ED                 PIC ZZ,ZZZ,ZZZ,ZZ9.99.

01  CAT-HEAD.
    05  FILLER                  PIC X(20)  VALUE "KATEGORI".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(7)   VALUE "    TRX".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(9)   VALUE "      QTY".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(17)  VALUE "         SUBTOTAL".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(16)  VALUE "          DISKON".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(17)  VALUE "        NET TOTAL".

01  CAT-LINE.
    05  CL-NAME                 PIC X(20).
    05  FILLER                  PIC X      VALUE SPACE.
    05  CL-TRX                  PIC ZZZ,ZZ9.
    05  FILLER                  PIC X      VALUE SPACE.
    05  CL-QTY                  PIC Z,ZZZ,ZZ9.
    05  FILLER                  PIC X      VALUE SPACE.
    05  CL-SUB                  PIC ZZ,ZZZ,ZZZ,ZZ9.99.
    05  FILLER                  PIC X      VALUE SPACE.
    05  CL-DISC                 PIC Z,ZZZ,ZZZ,ZZ9.99.
    05  FILLER                  PIC X      VALUE SPACE.
    05  CL-NET                  PIC ZZ,ZZZ,ZZZ,ZZ9.99.

01  DLY-HEAD.
    05  FILLER                  PIC X(10)  VALUE "TANGGAL".
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  FILLER                  PIC X(7)   VALUE "    TRX".
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  FILLER                  PIC X(9)   VALUE "      QTY".
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  FILLER                  PIC X(17)  VALUE "        NET TOTAL".

01  DLY-LINE.
    05  DL-DATE                 PIC X(10).
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  DL-TRX                  PIC ZZZ,ZZ9.
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  DL-QTY                  PIC Z,ZZZ,ZZ9.
    05  FILLER                  PIC X(2)   VALUE SPACES.
    05  DL-NET                  PIC ZZ,ZZZ,ZZZ,ZZ9.99.

01  PRD-HEAD.
    05  FILLER                  PIC X(4)   VALUE "NO".
    05  FILLER                  PIC X(32)  VALUE "PRODUK".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(18)  VALUE "KATEGORI".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(9)   VALUE "      QTY".
    05  FILLER                  PIC X      VALUE SPACE.
    05  FILLER                  PIC X(17)  VALUE "        NET TOTAL".

01  PRD-LINE.
    05  PL-RANK                 PIC Z9.
    05  FILLER                  PIC X(2)   VALUE ". ".
    05  PL-NAME                 PIC X(32).
    05  FILLER                  PIC X      VALUE SPACE.
    05  PL-CAT                  PIC X(18).
    05  FILLER                  PIC X      VALUE SPACE.
    05  PL-QTY                  PIC Z,ZZZ,ZZ9.
    05  FILLER                  PIC X      VALUE SPACE.
    05  PL-NET                  PIC ZZ,ZZZ,ZZZ,ZZ9.99.

PROCEDURE DIVISION.
MAIN-PROCEDURE.
    PERFORM RESOLVE-PATHS
    INITIALIZE CAT-TABLE
    INITIALIZE DLY-TABLE
    INITIALIZE PRD-TABLE

    OPEN INPUT IN-FILE
    IF WS-IN-STATUS NOT = "00"
        DISPLAY "ERROR: file input tidak ditemukan: "
            FUNCTION TRIM(WS-IN-PATH) " (status " WS-IN-STATUS ")"
        MOVE 1 TO RETURN-CODE
        STOP RUN
    END-IF

    PERFORM READ-NEXT-LINE
    IF END-OF-FILE
        DISPLAY "ERROR: file input kosong (header tidak ada)."
        CLOSE IN-FILE
        MOVE 2 TO RETURN-CODE
        STOP RUN
    END-IF
    IF IN-REC(1:14) NOT = "transaction_id"
        DISPLAY "ERROR: header CSV tidak valid."
        CLOSE IN-FILE
        MOVE 2 TO RETURN-CODE
        STOP RUN
    END-IF

    PERFORM READ-NEXT-LINE
    PERFORM UNTIL END-OF-FILE
        PERFORM PROCESS-RECORD
        PERFORM READ-NEXT-LINE
    END-PERFORM
    CLOSE IN-FILE

    PERFORM SORT-DAILY
    PERFORM SORT-PRODUCTS

    OPEN OUTPUT OUT-FILE
    IF WS-OUT-STATUS NOT = "00"
        DISPLAY "ERROR: gagal membuka file output: "
            FUNCTION TRIM(WS-OUT-PATH) " (status " WS-OUT-STATUS ")"
        MOVE 4 TO RETURN-CODE
        STOP RUN
    END-IF

    PERFORM WRITE-HEADER
    PERFORM WRITE-SECTION-CATEGORY
    PERFORM WRITE-SECTION-DAILY
    PERFORM WRITE-SECTION-TOP10
    PERFORM WRITE-FOOTER
    CLOSE OUT-FILE

    MOVE WS-REC-COUNT TO WS-COUNT-ED
    DISPLAY "COBOL REPORTER: " FUNCTION TRIM(WS-COUNT-ED)
        " record diproses."
    DISPLAY "Output: " FUNCTION TRIM(WS-OUT-PATH)
    MOVE 0 TO RETURN-CODE
    STOP RUN.

*> ---------------------------------------------------------------------
*> Menentukan path input/output: argumen CLI > env var > default
*> ---------------------------------------------------------------------
RESOLVE-PATHS.
    ACCEPT WS-CMD-LINE FROM COMMAND-LINE
    IF WS-CMD-LINE NOT = SPACES
        UNSTRING WS-CMD-LINE DELIMITED BY ALL SPACE
            INTO WS-ARG-IN WS-ARG-OUT
        END-UNSTRING
    END-IF

    IF WS-ARG-IN NOT = SPACES
        MOVE WS-ARG-IN TO WS-IN-PATH
    ELSE
        ACCEPT WS-IN-PATH FROM ENVIRONMENT "INPUT_CSV_PATH"
    END-IF
    IF WS-IN-PATH = SPACES
        MOVE "data/processed_data.csv" TO WS-IN-PATH
    END-IF

    IF WS-ARG-OUT NOT = SPACES
        MOVE WS-ARG-OUT TO WS-OUT-PATH
    ELSE
        ACCEPT WS-OUT-PATH FROM ENVIRONMENT "OUTPUT_REPORT_PATH"
    END-IF
    IF WS-OUT-PATH = SPACES
        MOVE "data/report_output.txt" TO WS-OUT-PATH
    END-IF.

*> ---------------------------------------------------------------------
READ-NEXT-LINE.
    MOVE SPACES TO IN-REC
    READ IN-FILE
        AT END
            SET END-OF-FILE TO TRUE
        NOT AT END
            ADD 1 TO WS-LINE-NO
    END-READ.

*> ---------------------------------------------------------------------
*> Memecah satu record CSV, validasi, lalu akumulasi ke semua tabel
*> ---------------------------------------------------------------------
PROCESS-RECORD.
    INSPECT IN-REC REPLACING ALL X"0D" BY SPACE
    IF IN-REC NOT = SPACES
        MOVE SPACES TO WS-FIELDS
        MOVE 0 TO WS-FLD-COUNT
        UNSTRING IN-REC DELIMITED BY ","
            INTO F-ID F-NAME F-CAT F-QTY F-SUB F-DISC F-NET
                 F-PAY F-DATE
            TALLYING IN WS-FLD-COUNT
        END-UNSTRING

        IF WS-FLD-COUNT < 9
           OR F-NAME = SPACES
           OR F-CAT = SPACES
           OR F-DATE = SPACES
           OR FUNCTION TEST-NUMVAL(F-QTY) NOT = 0
           OR FUNCTION TEST-NUMVAL(F-SUB) NOT = 0
           OR FUNCTION TEST-NUMVAL(F-DISC) NOT = 0
           OR FUNCTION TEST-NUMVAL(F-NET) NOT = 0
            MOVE WS-LINE-NO TO WS-LINE-NO-ED
            DISPLAY "ERROR: format record tidak valid pada baris "
                FUNCTION TRIM(WS-LINE-NO-ED)
            CLOSE IN-FILE
            MOVE 2 TO RETURN-CODE
            STOP RUN
        END-IF

        COMPUTE N-QTY  = FUNCTION NUMVAL(F-QTY)
        COMPUTE N-SUB  = FUNCTION NUMVAL(F-SUB)
        COMPUTE N-DISC = FUNCTION NUMVAL(F-DISC)
        COMPUTE N-NET  = FUNCTION NUMVAL(F-NET)

        ADD 1      TO WS-REC-COUNT
        ADD N-QTY  TO WS-TOT-QTY
        ADD N-SUB  TO WS-TOT-SUB
        ADD N-DISC TO WS-TOT-DISC
        ADD N-NET  TO WS-TOT-NET

        MOVE F-CAT  TO WS-KEY-CAT
        MOVE F-DATE TO WS-KEY-DATE
        MOVE F-NAME TO WS-KEY-NAME

        IF WS-KEY-DATE < WS-MIN-DATE
            MOVE WS-KEY-DATE TO WS-MIN-DATE
        END-IF
        IF WS-KEY-DATE > WS-MAX-DATE
            MOVE WS-KEY-DATE TO WS-MAX-DATE
        END-IF

        PERFORM ACCUMULATE-CATEGORY
        PERFORM ACCUMULATE-DAILY
        PERFORM ACCUMULATE-PRODUCT
    END-IF.

*> ---------------------------------------------------------------------
ACCUMULATE-CATEGORY.
    MOVE 0 TO WS-FOUND-IDX
    PERFORM VARYING WS-I FROM 1 BY 1
            UNTIL WS-I > WS-CAT-COUNT OR WS-FOUND-IDX > 0
        IF CAT-NAME(WS-I) = WS-KEY-CAT
            MOVE WS-I TO WS-FOUND-IDX
        END-IF
    END-PERFORM
    IF WS-FOUND-IDX = 0
        IF WS-CAT-COUNT < 50
            ADD 1 TO WS-CAT-COUNT
            MOVE WS-CAT-COUNT TO WS-FOUND-IDX
            MOVE WS-KEY-CAT TO CAT-NAME(WS-FOUND-IDX)
            MOVE 0 TO CAT-TRX(WS-FOUND-IDX)
            MOVE 0 TO CAT-QTY(WS-FOUND-IDX)
            MOVE 0 TO CAT-SUB(WS-FOUND-IDX)
            MOVE 0 TO CAT-DISC(WS-FOUND-IDX)
            MOVE 0 TO CAT-NET(WS-FOUND-IDX)
        ELSE
            ADD 1 TO WS-SKIPPED
        END-IF
    END-IF
    IF WS-FOUND-IDX > 0
        ADD 1      TO CAT-TRX(WS-FOUND-IDX)
        ADD N-QTY  TO CAT-QTY(WS-FOUND-IDX)
        ADD N-SUB  TO CAT-SUB(WS-FOUND-IDX)
        ADD N-DISC TO CAT-DISC(WS-FOUND-IDX)
        ADD N-NET  TO CAT-NET(WS-FOUND-IDX)
    END-IF.

*> ---------------------------------------------------------------------
ACCUMULATE-DAILY.
    MOVE 0 TO WS-FOUND-IDX
    PERFORM VARYING WS-I FROM 1 BY 1
            UNTIL WS-I > WS-DLY-COUNT OR WS-FOUND-IDX > 0
        IF DLY-DATE(WS-I) = WS-KEY-DATE
            MOVE WS-I TO WS-FOUND-IDX
        END-IF
    END-PERFORM
    IF WS-FOUND-IDX = 0
        IF WS-DLY-COUNT < 400
            ADD 1 TO WS-DLY-COUNT
            MOVE WS-DLY-COUNT TO WS-FOUND-IDX
            MOVE WS-KEY-DATE TO DLY-DATE(WS-FOUND-IDX)
            MOVE 0 TO DLY-TRX(WS-FOUND-IDX)
            MOVE 0 TO DLY-QTY(WS-FOUND-IDX)
            MOVE 0 TO DLY-NET(WS-FOUND-IDX)
        ELSE
            ADD 1 TO WS-SKIPPED
        END-IF
    END-IF
    IF WS-FOUND-IDX > 0
        ADD 1     TO DLY-TRX(WS-FOUND-IDX)
        ADD N-QTY TO DLY-QTY(WS-FOUND-IDX)
        ADD N-NET TO DLY-NET(WS-FOUND-IDX)
    END-IF.

*> ---------------------------------------------------------------------
ACCUMULATE-PRODUCT.
    MOVE 0 TO WS-FOUND-IDX
    PERFORM VARYING WS-I FROM 1 BY 1
            UNTIL WS-I > WS-PRD-COUNT OR WS-FOUND-IDX > 0
        IF PRD-NAME(WS-I) = WS-KEY-NAME
            MOVE WS-I TO WS-FOUND-IDX
        END-IF
    END-PERFORM
    IF WS-FOUND-IDX = 0
        IF WS-PRD-COUNT < 500
            ADD 1 TO WS-PRD-COUNT
            MOVE WS-PRD-COUNT TO WS-FOUND-IDX
            MOVE WS-KEY-NAME TO PRD-NAME(WS-FOUND-IDX)
            MOVE WS-KEY-CAT  TO PRD-CAT(WS-FOUND-IDX)
            MOVE 0 TO PRD-QTY(WS-FOUND-IDX)
            MOVE 0 TO PRD-NET(WS-FOUND-IDX)
        ELSE
            ADD 1 TO WS-SKIPPED
        END-IF
    END-IF
    IF WS-FOUND-IDX > 0
        ADD N-QTY TO PRD-QTY(WS-FOUND-IDX)
        ADD N-NET TO PRD-NET(WS-FOUND-IDX)
    END-IF.

*> ---------------------------------------------------------------------
*> Bubble sort: tabel harian naik berdasarkan tanggal
*> ---------------------------------------------------------------------
SORT-DAILY.
    PERFORM VARYING WS-I FROM 1 BY 1 UNTIL WS-I >= WS-DLY-COUNT
        COMPUTE WS-LIMIT = WS-DLY-COUNT - WS-I
        PERFORM VARYING WS-J FROM 1 BY 1 UNTIL WS-J > WS-LIMIT
            IF DLY-DATE(WS-J) > DLY-DATE(WS-J + 1)
                MOVE DLY-ENTRY(WS-J)     TO DLY-SWAP
                MOVE DLY-ENTRY(WS-J + 1) TO DLY-ENTRY(WS-J)
                MOVE DLY-SWAP            TO DLY-ENTRY(WS-J + 1)
            END-IF
        END-PERFORM
    END-PERFORM.

*> ---------------------------------------------------------------------
*> Bubble sort: tabel produk turun berdasarkan qty, lalu net total
*> ---------------------------------------------------------------------
SORT-PRODUCTS.
    PERFORM VARYING WS-I FROM 1 BY 1 UNTIL WS-I >= WS-PRD-COUNT
        COMPUTE WS-LIMIT = WS-PRD-COUNT - WS-I
        PERFORM VARYING WS-J FROM 1 BY 1 UNTIL WS-J > WS-LIMIT
            IF PRD-QTY(WS-J) < PRD-QTY(WS-J + 1)
               OR (PRD-QTY(WS-J) = PRD-QTY(WS-J + 1)
                   AND PRD-NET(WS-J) < PRD-NET(WS-J + 1))
                MOVE PRD-ENTRY(WS-J)     TO PRD-SWAP
                MOVE PRD-ENTRY(WS-J + 1) TO PRD-ENTRY(WS-J)
                MOVE PRD-SWAP            TO PRD-ENTRY(WS-J + 1)
            END-IF
        END-PERFORM
    END-PERFORM.

*> ---------------------------------------------------------------------
*> Utilitas penulisan baris
*> ---------------------------------------------------------------------
PUT-LINE.
    WRITE OUT-REC
    IF WS-OUT-STATUS NOT = "00"
        DISPLAY "ERROR: gagal menulis file output (status "
            WS-OUT-STATUS ")"
        CLOSE OUT-FILE
        MOVE 4 TO RETURN-CODE
        STOP RUN
    END-IF.

PUT-BLANK.
    MOVE SPACES TO OUT-REC
    PERFORM PUT-LINE.

PUT-SEP-EQ.
    MOVE WS-SEP-EQ TO OUT-REC
    PERFORM PUT-LINE.

PUT-SEP-DASH.
    MOVE WS-SEP-DASH TO OUT-REC
    PERFORM PUT-LINE.

*> ---------------------------------------------------------------------
WRITE-HEADER.
    MOVE FUNCTION CURRENT-DATE TO WS-NOW
    PERFORM PUT-SEP-EQ
    MOVE "                         LAPORAN PENJUALAN - POINT OF SALES (POS)"
        TO OUT-REC
    PERFORM PUT-LINE
    MOVE "                         Dibuat oleh COBOL Batch Reporting Engine"
        TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-EQ

    MOVE SPACES TO OUT-REC
    STRING "Tanggal cetak  : " WS-NOW-YYYY "-" WS-NOW-MM "-" WS-NOW-DD
           " " WS-NOW-HH ":" WS-NOW-MI ":" WS-NOW-SS
        DELIMITED BY SIZE INTO OUT-REC
    END-STRING
    PERFORM PUT-LINE

    MOVE SPACES TO OUT-REC
    IF WS-REC-COUNT > 0
        STRING "Periode data   : " WS-MIN-DATE " s/d " WS-MAX-DATE
            DELIMITED BY SIZE INTO OUT-REC
        END-STRING
    ELSE
        MOVE "Periode data   : -" TO OUT-REC
    END-IF
    PERFORM PUT-LINE

    MOVE WS-REC-COUNT TO WS-COUNT-ED
    MOVE SPACES TO OUT-REC
    STRING "Jumlah record  : " FUNCTION TRIM(WS-COUNT-ED) " transaksi"
        DELIMITED BY SIZE INTO OUT-REC
    END-STRING
    PERFORM PUT-LINE
    PERFORM PUT-BLANK.

*> ---------------------------------------------------------------------
WRITE-SECTION-CATEGORY.
    MOVE "A. REKAPITULASI PENDAPATAN PER KATEGORI" TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH
    MOVE CAT-HEAD TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH

    IF WS-CAT-COUNT = 0
        MOVE "(tidak ada data)" TO OUT-REC
        PERFORM PUT-LINE
    END-IF
    PERFORM VARYING WS-I FROM 1 BY 1 UNTIL WS-I > WS-CAT-COUNT
        MOVE CAT-NAME(WS-I) TO CL-NAME
        MOVE CAT-TRX(WS-I)  TO CL-TRX
        MOVE CAT-QTY(WS-I)  TO CL-QTY
        MOVE CAT-SUB(WS-I)  TO CL-SUB
        MOVE CAT-DISC(WS-I) TO CL-DISC
        MOVE CAT-NET(WS-I)  TO CL-NET
        MOVE CAT-LINE TO OUT-REC
        PERFORM PUT-LINE
    END-PERFORM

    PERFORM PUT-SEP-DASH
    MOVE "GRAND TOTAL"  TO CL-NAME
    MOVE WS-REC-COUNT   TO CL-TRX
    MOVE WS-TOT-QTY     TO CL-QTY
    MOVE WS-TOT-SUB     TO CL-SUB
    MOVE WS-TOT-DISC    TO CL-DISC
    MOVE WS-TOT-NET     TO CL-NET
    MOVE CAT-LINE TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH

    IF WS-REC-COUNT > 0
        COMPUTE WS-AVG-NET ROUNDED = WS-TOT-NET / WS-REC-COUNT
    ELSE
        MOVE 0 TO WS-AVG-NET
    END-IF
    MOVE WS-AVG-NET TO WS-MONEY-ED
    MOVE SPACES TO OUT-REC
    STRING "Rata-rata net per transaksi : " FUNCTION TRIM(WS-MONEY-ED)
        DELIMITED BY SIZE INTO OUT-REC
    END-STRING
    PERFORM PUT-LINE
    PERFORM PUT-BLANK.

*> ---------------------------------------------------------------------
WRITE-SECTION-DAILY.
    MOVE "B. LAPORAN PENJUALAN HARIAN" TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH
    MOVE DLY-HEAD TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH

    IF WS-DLY-COUNT = 0
        MOVE "(tidak ada data)" TO OUT-REC
        PERFORM PUT-LINE
    END-IF
    PERFORM VARYING WS-I FROM 1 BY 1 UNTIL WS-I > WS-DLY-COUNT
        MOVE DLY-DATE(WS-I) TO DL-DATE
        MOVE DLY-TRX(WS-I)  TO DL-TRX
        MOVE DLY-QTY(WS-I)  TO DL-QTY
        MOVE DLY-NET(WS-I)  TO DL-NET
        MOVE DLY-LINE TO OUT-REC
        PERFORM PUT-LINE
    END-PERFORM

    PERFORM PUT-SEP-DASH
    MOVE "TOTAL"      TO DL-DATE
    MOVE WS-REC-COUNT TO DL-TRX
    MOVE WS-TOT-QTY   TO DL-QTY
    MOVE WS-TOT-NET   TO DL-NET
    MOVE DLY-LINE TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-BLANK.

*> ---------------------------------------------------------------------
WRITE-SECTION-TOP10.
    MOVE "C. LAPORAN PRODUK TERLARIS (TOP 10)" TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH
    MOVE PRD-HEAD TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-DASH

    IF WS-PRD-COUNT = 0
        MOVE "(tidak ada data)" TO OUT-REC
        PERFORM PUT-LINE
    END-IF
    PERFORM VARYING WS-I FROM 1 BY 1
            UNTIL WS-I > WS-PRD-COUNT OR WS-I > 10
        MOVE WS-I           TO PL-RANK
        MOVE PRD-NAME(WS-I) TO PL-NAME
        MOVE PRD-CAT(WS-I)  TO PL-CAT
        MOVE PRD-QTY(WS-I)  TO PL-QTY
        MOVE PRD-NET(WS-I)  TO PL-NET
        MOVE PRD-LINE TO OUT-REC
        PERFORM PUT-LINE
    END-PERFORM
    PERFORM PUT-BLANK.

*> ---------------------------------------------------------------------
WRITE-FOOTER.
    PERFORM PUT-SEP-EQ
    IF WS-SKIPPED > 0
        MOVE "CATATAN: sebagian data melebihi kapasitas tabel laporan."
            TO OUT-REC
        PERFORM PUT-LINE
    END-IF
    MOVE "                                   *** AKHIR LAPORAN ***"
        TO OUT-REC
    PERFORM PUT-LINE
    PERFORM PUT-SEP-EQ.
