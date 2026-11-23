/*
 * processor.c - POS Transaction Processing Engine (Bahasa C)
 *
 * Pemakaian : processor <input transactions.csv> <output processed_data.csv>
 *
 * Alur      : baca CSV -> validasi -> kalkulasi (subtotal, discount_amount,
 *             net_total) -> sorting berdasarkan transaction_date -> tulis CSV
 *             untuk COBOL -> cetak statistik ke stdout (baris "STAT key=value"
 *             dibaca oleh backend Go).
 *
 * Exit code : 0 sukses | 1 file input tidak ditemukan | 2 format tidak valid
 *             3 gagal alokasi memori | 4 gagal menulis file output
 *
 * Memori    : seluruh buffer baris, array field, dan array transaksi
 *             dialokasikan dinamis (malloc/calloc/realloc), setiap pointer
 *             dicek terhadap NULL, dan dibebaskan dengan free() lalu di-set NULL.
 */
#include "processor.h"

#include <ctype.h>
#include <errno.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

/* ------------------------------------------------------------------ */
/* Utilitas kecil                                                      */
/* ------------------------------------------------------------------ */

/* Salin string dengan aman: selalu diakhiri '\0'. */
static void safe_copy(char *dest, const char *src, size_t dest_size)
{
    if (dest == NULL || dest_size == 0) {
        return;
    }
    if (src == NULL) {
        dest[0] = '\0';
        return;
    }
    /* strncpy tidak menjamin terminator bila sumber terlalu panjang,
     * maka byte terakhir selalu diisi '\0'. */
    strncpy(dest, src, dest_size - 1);
    dest[dest_size - 1] = '\0';
}

/* Salin ID antar buffer berukuran sama (ID_LEN) yang sudah ber-terminator. */
static void copy_id(char *dest, const char *src)
{
    memcpy(dest, src, ID_LEN);
    dest[ID_LEN - 1] = '\0';
}

/* Buang spasi di awal & akhir (in-place). */
static void trim(char *s)
{
    size_t len;
    size_t start = 0;

    if (s == NULL) {
        return;
    }
    len = strlen(s);
    while (len > 0 && isspace((unsigned char)s[len - 1])) {
        s[--len] = '\0';
    }
    while (s[start] != '\0' && isspace((unsigned char)s[start])) {
        start++;
    }
    if (start > 0) {
        memmove(s, s + start, len - start + 1);
    }
}

/* Pembulatan 2 desimal untuk nilai uang. */
static double round2(double value)
{
    return floor(value * 100.0 + 0.5) / 100.0;
}

/* Parsing bilangan bulat secara ketat. Return 1 jika valid. */
static int parse_int_strict(const char *s, int *out)
{
    char *end = NULL;
    long  value;

    if (s == NULL || *s == '\0') {
        return 0;
    }
    errno = 0;
    value = strtol(s, &end, 10);
    if (errno != 0 || end == s || *end != '\0') {
        return 0;
    }
    if (value < -2000000000L || value > 2000000000L) {
        return 0;
    }
    *out = (int)value;
    return 1;
}

/* Parsing bilangan desimal secara ketat. Return 1 jika valid. */
static int parse_double_strict(const char *s, double *out)
{
    char  *end = NULL;
    double value;

    if (s == NULL || *s == '\0') {
        return 0;
    }
    errno = 0;
    value = strtod(s, &end);
    if (errno != 0 || end == s || *end != '\0') {
        return 0;
    }
    if (value != value || value > 1e15 || value < -1e15) { /* NaN / di luar jangkauan */
        return 0;
    }
    *out = value;
    return 1;
}

/* Validasi tanggal format YYYY-MM-DD. Return 1 jika valid. */
static int is_valid_date(const char *s)
{
    static const int days_in_month[12] = {31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31};
    int year, month, day, max_day, i;

    if (s == NULL || strlen(s) != 10) {
        return 0;
    }
    for (i = 0; i < 10; i++) {
        if (i == 4 || i == 7) {
            if (s[i] != '-') {
                return 0;
            }
        } else if (!isdigit((unsigned char)s[i])) {
            return 0;
        }
    }
    year  = (s[0] - '0') * 1000 + (s[1] - '0') * 100 + (s[2] - '0') * 10 + (s[3] - '0');
    month = (s[5] - '0') * 10 + (s[6] - '0');
    day   = (s[8] - '0') * 10 + (s[9] - '0');
    if (year < 1900 || month < 1 || month > 12 || day < 1) {
        return 0;
    }
    max_day = days_in_month[month - 1];
    if (month == 2 && ((year % 4 == 0 && year % 100 != 0) || year % 400 == 0)) {
        max_day = 29;
    }
    return day <= max_day;
}

/* Field output tidak boleh memuat koma/kutip/baris baru karena COBOL
 * memecah record dengan UNSTRING ... DELIMITED BY ",". */
static void sanitize_for_output(char *s)
{
    char *start = s;

    if (s == NULL) {
        return;
    }
    for (; *s != '\0'; s++) {
        if (*s == ',' || *s == '"' || *s == '\r' || *s == '\n') {
            *s = ' ';
        }
    }
    trim(start);
}

/* ------------------------------------------------------------------ */
/* Manajemen memori dinamis: TransactionList                           */
/* ------------------------------------------------------------------ */

/* Alokasi awal array transaksi dengan calloc (memori ter-nol-kan). */
int list_init(TransactionList *list, size_t initial_capacity)
{
    if (list == NULL) {
        return EXIT_NO_MEMORY;
    }
    if (initial_capacity == 0) {
        initial_capacity = INITIAL_CAPACITY;
    }
    list->count    = 0;
    list->capacity = 0;
    list->items    = (Transaction *)calloc(initial_capacity, sizeof(Transaction));
    if (list->items == NULL) {
        return EXIT_NO_MEMORY;
    }
    list->capacity = initial_capacity;
    return EXIT_OK;
}

/* Tambah satu record; kapasitas digandakan dengan realloc bila penuh. */
int list_push(TransactionList *list, const Transaction *trx)
{
    if (list == NULL || trx == NULL || list->items == NULL) {
        return EXIT_NO_MEMORY;
    }
    if (list->count >= list->capacity) {
        size_t       new_capacity = list->capacity * 2;
        Transaction *resized;

        if (new_capacity <= list->capacity ||
            new_capacity > ((size_t)-1) / sizeof(Transaction)) {
            return EXIT_NO_MEMORY; /* overflow ukuran */
        }
        /* Pakai pointer sementara: jika realloc gagal, blok lama tetap
         * valid dan masih bisa dibebaskan oleh list_free(). */
        resized = (Transaction *)realloc(list->items, new_capacity * sizeof(Transaction));
        if (resized == NULL) {
            return EXIT_NO_MEMORY;
        }
        list->items    = resized;
        list->capacity = new_capacity;
    }
    list->items[list->count] = *trx;
    list->count++;
    return EXIT_OK;
}

/* Bebaskan array transaksi dan set pointer ke NULL. */
void list_free(TransactionList *list)
{
    if (list == NULL) {
        return;
    }
    if (list->items != NULL) {
        free(list->items);
        list->items = NULL;
    }
    list->count    = 0;
    list->capacity = 0;
}

/* ------------------------------------------------------------------ */
/* Pembacaan baris & parsing CSV                                       */
/* ------------------------------------------------------------------ */

/*
 * Baca satu baris dengan panjang berapa pun (buffer tumbuh via realloc).
 * Return: pointer baris (harus di-free pemanggil) atau NULL.
 * *status: EXIT_OK (baris terbaca / EOF) atau EXIT_NO_MEMORY.
 */
char *read_line(FILE *fp, int *status)
{
    size_t capacity = 256;
    size_t length   = 0;
    char  *buffer;
    int    ch;

    *status = EXIT_OK;
    buffer  = (char *)malloc(capacity);
    if (buffer == NULL) {
        *status = EXIT_NO_MEMORY;
        return NULL;
    }

    while ((ch = fgetc(fp)) != EOF) {
        if (ch == '\n') {
            break;
        }
        if (length + 1 >= capacity) {
            size_t new_capacity = capacity * 2;
            char  *resized      = (char *)realloc(buffer, new_capacity);
            if (resized == NULL) {
                free(buffer);
                buffer  = NULL;
                *status = EXIT_NO_MEMORY;
                return NULL;
            }
            buffer   = resized;
            capacity = new_capacity;
        }
        buffer[length++] = (char)ch;
    }

    if (ch == EOF && length == 0) { /* akhir file, tidak ada data */
        free(buffer);
        buffer = NULL;
        return NULL;
    }

    if (length > 0 && buffer[length - 1] == '\r') { /* baris gaya Windows */
        length--;
    }
    buffer[length] = '\0';
    return buffer;
}

/* Bebaskan array field hasil split_csv_line dan set pointer ke NULL. */
void free_fields(char ***fields, size_t count)
{
    size_t i;

    if (fields == NULL || *fields == NULL) {
        return;
    }
    for (i = 0; i < count; i++) {
        if ((*fields)[i] != NULL) {
            free((*fields)[i]);
            (*fields)[i] = NULL;
        }
    }
    free(*fields);
    *fields = NULL;
}

/*
 * Pecah satu baris CSV (mendukung field ber-kutip "..." dan kutip ganda "").
 * Hasil: array string dinamis. Return EXIT_OK, EXIT_BAD_FORMAT, atau
 * EXIT_NO_MEMORY. Pemanggil wajib memanggil free_fields().
 */
int split_csv_line(const char *line, char ***fields_out, size_t *count_out)
{
    size_t      line_len = strlen(line);
    size_t      capacity = INPUT_FIELD_COUNT;
    size_t      count    = 0;
    char      **fields;
    const char *p = line;

    *fields_out = NULL;
    *count_out  = 0;

    fields = (char **)calloc(capacity, sizeof(char *));
    if (fields == NULL) {
        return EXIT_NO_MEMORY;
    }

    for (;;) {
        size_t length = 0;
        char  *field  = (char *)malloc(line_len + 1); /* field tak mungkin lebih panjang dari baris */

        if (field == NULL) {
            free_fields(&fields, count);
            return EXIT_NO_MEMORY;
        }

        if (*p == '"') { /* field ber-kutip */
            p++;
            for (;;) {
                if (*p == '\0') { /* kutip tidak ditutup */
                    free(field);
                    field = NULL;
                    free_fields(&fields, count);
                    return EXIT_BAD_FORMAT;
                }
                if (*p == '"') {
                    if (p[1] == '"') {
                        field[length++] = '"';
                        p += 2;
                        continue;
                    }
                    p++;
                    break;
                }
                field[length++] = *p++;
            }
            if (*p != ',' && *p != '\0') { /* karakter liar setelah kutip penutup */
                free(field);
                field = NULL;
                free_fields(&fields, count);
                return EXIT_BAD_FORMAT;
            }
        } else {
            while (*p != ',' && *p != '\0') {
                field[length++] = *p++;
            }
        }
        field[length] = '\0';
        trim(field);

        if (count >= capacity) {
            size_t  new_capacity = capacity * 2;
            char  **resized      = (char **)realloc(fields, new_capacity * sizeof(char *));
            if (resized == NULL) {
                free(field);
                field = NULL;
                free_fields(&fields, count);
                return EXIT_NO_MEMORY;
            }
            fields   = resized;
            capacity = new_capacity;
        }
        fields[count++] = field;

        if (*p == ',') {
            p++;
            continue;
        }
        break;
    }

    *fields_out = fields;
    *count_out  = count;
    return EXIT_OK;
}

/* Validasi & konversi array field menjadi struct Transaction. */
int parse_transaction(char **fields, size_t count, Transaction *out)
{
    if (fields == NULL || out == NULL || count != INPUT_FIELD_COUNT) {
        return EXIT_BAD_FORMAT;
    }
    memset(out, 0, sizeof(*out));

    if (fields[0][0] == '\0' || strlen(fields[0]) > 10 || strlen(fields[0]) >= ID_LEN ||
        fields[1][0] == '\0' || strlen(fields[1]) > 10 || strlen(fields[1]) >= ID_LEN ||
        fields[2][0] == '\0' || strlen(fields[2]) >= NAME_LEN ||
        fields[3][0] == '\0' || strlen(fields[3]) >= CATEGORY_LEN ||
        fields[7][0] == '\0' || strlen(fields[7]) >= PAYMENT_LEN ||
        fields[9][0] == '\0' || strlen(fields[9]) >= CASHIER_LEN) {
        return EXIT_BAD_FORMAT;
    }
    if (!parse_int_strict(fields[4], &out->quantity) || out->quantity <= 0) {
        return EXIT_BAD_FORMAT;
    }
    if (!parse_double_strict(fields[5], &out->unit_price) || out->unit_price <= 0.0) {
        return EXIT_BAD_FORMAT;
    }
    if (!parse_double_strict(fields[6], &out->discount_percent) ||
        out->discount_percent < 0.0 || out->discount_percent > 100.0) {
        return EXIT_BAD_FORMAT;
    }
    if (!is_valid_date(fields[8])) {
        return EXIT_BAD_FORMAT;
    }
    if (strcmp(fields[7], "Cash") != 0 && strcmp(fields[7], "Debit") != 0 &&
        strcmp(fields[7], "Credit") != 0) {
        return EXIT_BAD_FORMAT;
    }

    safe_copy(out->transaction_id, fields[0], sizeof(out->transaction_id));
    safe_copy(out->product_id, fields[1], sizeof(out->product_id));
    safe_copy(out->product_name, fields[2], sizeof(out->product_name));
    safe_copy(out->category, fields[3], sizeof(out->category));
    safe_copy(out->payment_method, fields[7], sizeof(out->payment_method));
    safe_copy(out->transaction_date, fields[8], sizeof(out->transaction_date));
    safe_copy(out->cashier_name, fields[9], sizeof(out->cashier_name));
    return EXIT_OK;
}

/*
 * Muat seluruh transaksi dari file CSV ke array dinamis.
 * *bad_line diisi nomor baris bermasalah (untuk pesan error).
 */
int load_transactions(const char *path, TransactionList *list, size_t *bad_line)
{
    FILE  *fp;
    char  *line    = NULL;
    size_t line_no = 0;
    int    status  = EXIT_OK;
    int    result  = EXIT_OK;

    *bad_line = 0;
    fp = fopen(path, "r");
    if (fp == NULL) {
        return EXIT_FILE_NOTFOUND;
    }

    /* --- baris header --- */
    line = read_line(fp, &status);
    line_no++;
    if (line == NULL) {
        fclose(fp);
        *bad_line = line_no;
        return status == EXIT_NO_MEMORY ? EXIT_NO_MEMORY : EXIT_BAD_FORMAT; /* file kosong */
    }
    {
        char *header = line;
        /* lewati UTF-8 BOM bila ada */
        if (strlen(header) >= 3 && (unsigned char)header[0] == 0xEF &&
            (unsigned char)header[1] == 0xBB && (unsigned char)header[2] == 0xBF) {
            header += 3;
        }
        trim(header);
        if (strcmp(header, INPUT_HEADER) != 0) {
            result = EXIT_BAD_FORMAT;
        }
    }
    free(line);
    line = NULL;
    if (result != EXIT_OK) {
        fclose(fp);
        *bad_line = line_no;
        return result;
    }

    /* --- baris data --- */
    for (;;) {
        char      **fields = NULL;
        size_t      count  = 0;
        Transaction trx;
        const char *probe;

        line = read_line(fp, &status);
        if (line == NULL) {
            if (status != EXIT_OK) {
                result = status;
            }
            break;
        }
        line_no++;

        /* lewati baris kosong */
        probe = line;
        while (*probe != '\0' && isspace((unsigned char)*probe)) {
            probe++;
        }
        if (*probe == '\0') {
            free(line);
            line = NULL;
            continue;
        }

        result = split_csv_line(line, &fields, &count);
        free(line);
        line = NULL;
        if (result != EXIT_OK) {
            *bad_line = line_no;
            break;
        }

        result = parse_transaction(fields, count, &trx);
        free_fields(&fields, count);
        if (result != EXIT_OK) {
            *bad_line = line_no;
            break;
        }

        trx.sequence = list->count;
        result = list_push(list, &trx);
        if (result != EXIT_OK) {
            *bad_line = line_no;
            break;
        }
    }

    fclose(fp);
    return result;
}

/* ------------------------------------------------------------------ */
/* Kalkulasi, sorting, statistik                                       */
/* ------------------------------------------------------------------ */

void calculate_transaction(Transaction *trx)
{
    if (trx == NULL) {
        return;
    }
    trx->subtotal        = round2((double)trx->quantity * trx->unit_price);
    trx->discount_amount = round2(trx->subtotal * trx->discount_percent / 100.0);
    trx->net_total       = round2(trx->subtotal - trx->discount_amount);
}

void calculate_all(TransactionList *list)
{
    size_t i;

    if (list == NULL || list->items == NULL) {
        return;
    }
    for (i = 0; i < list->count; i++) {
        calculate_transaction(&list->items[i]);
    }
}

/* Pembanding qsort: tanggal naik, lalu urutan baca (stabil). */
static int compare_by_date(const void *a, const void *b)
{
    const Transaction *left  = (const Transaction *)a;
    const Transaction *right = (const Transaction *)b;
    int                cmp   = strcmp(left->transaction_date, right->transaction_date);

    if (cmp != 0) {
        return cmp;
    }
    if (left->sequence < right->sequence) {
        return -1;
    }
    if (left->sequence > right->sequence) {
        return 1;
    }
    return 0;
}

void sort_by_date(TransactionList *list)
{
    if (list == NULL || list->items == NULL || list->count < 2) {
        return;
    }
    qsort(list->items, list->count, sizeof(Transaction), compare_by_date);
}

void compute_statistics(const TransactionList *list, Statistics *stats)
{
    size_t i;

    if (stats == NULL) {
        return;
    }
    memset(stats, 0, sizeof(*stats));
    if (list == NULL || list->items == NULL || list->count == 0) {
        return;
    }

    stats->total_records = list->count;
    stats->max_net       = list->items[0].net_total;
    stats->min_net       = list->items[0].net_total;
    copy_id(stats->max_id, list->items[0].transaction_id);
    copy_id(stats->min_id, list->items[0].transaction_id);

    for (i = 0; i < list->count; i++) {
        const Transaction *trx = &list->items[i];

        stats->total_quantity += trx->quantity;
        stats->total_subtotal += trx->subtotal;
        stats->total_discount += trx->discount_amount;
        stats->total_net      += trx->net_total;

        if (trx->net_total > stats->max_net) {
            stats->max_net = trx->net_total;
            copy_id(stats->max_id, trx->transaction_id);
        }
        if (trx->net_total < stats->min_net) {
            stats->min_net = trx->net_total;
            copy_id(stats->min_id, trx->transaction_id);
        }

        if (strcmp(trx->payment_method, "Cash") == 0) {
            stats->cash_count++;
            stats->cash_total += trx->net_total;
        } else if (strcmp(trx->payment_method, "Debit") == 0) {
            stats->debit_count++;
            stats->debit_total += trx->net_total;
        } else if (strcmp(trx->payment_method, "Credit") == 0) {
            stats->credit_count++;
            stats->credit_total += trx->net_total;
        }
    }
    stats->average_net = stats->total_net / (double)stats->total_records;
}

/* ------------------------------------------------------------------ */
/* Output                                                              */
/* ------------------------------------------------------------------ */

int write_processed(const char *path, const TransactionList *list)
{
    FILE  *fp;
    size_t i;

    fp = fopen(path, "w");
    if (fp == NULL) {
        return EXIT_WRITE_FAILED;
    }
    if (fprintf(fp, "%s\n", OUTPUT_HEADER) < 0) {
        fclose(fp);
        return EXIT_WRITE_FAILED;
    }
    for (i = 0; i < list->count; i++) {
        Transaction row = list->items[i]; /* salinan, agar data asli tidak berubah */

        sanitize_for_output(row.transaction_id);
        sanitize_for_output(row.product_name);
        sanitize_for_output(row.category);
        sanitize_for_output(row.payment_method);

        if (fprintf(fp, "%s,%s,%s,%d,%.2f,%.2f,%.2f,%s,%s\n",
                    row.transaction_id, row.product_name, row.category, row.quantity,
                    row.subtotal, row.discount_amount, row.net_total,
                    row.payment_method, row.transaction_date) < 0) {
            fclose(fp);
            return EXIT_WRITE_FAILED;
        }
    }
    if (fclose(fp) != 0) {
        return EXIT_WRITE_FAILED;
    }
    return EXIT_OK;
}

void print_statistics(const Statistics *stats)
{
    printf("=== C ENGINE: RINGKASAN PEMROSESAN ===\n");
    printf("STAT total_records=%lu\n", (unsigned long)stats->total_records);
    printf("STAT total_quantity=%ld\n", stats->total_quantity);
    printf("STAT total_subtotal=%.2f\n", stats->total_subtotal);
    printf("STAT total_discount=%.2f\n", stats->total_discount);
    printf("STAT total_net=%.2f\n", stats->total_net);
    printf("STAT average_net=%.2f\n", stats->average_net);
    printf("STAT max_net=%.2f\n", stats->max_net);
    printf("STAT max_transaction_id=%s\n", stats->max_id);
    printf("STAT min_net=%.2f\n", stats->min_net);
    printf("STAT min_transaction_id=%s\n", stats->min_id);
    printf("STAT cash_count=%lu\n", (unsigned long)stats->cash_count);
    printf("STAT cash_total=%.2f\n", stats->cash_total);
    printf("STAT debit_count=%lu\n", (unsigned long)stats->debit_count);
    printf("STAT debit_total=%.2f\n", stats->debit_total);
    printf("STAT credit_count=%lu\n", (unsigned long)stats->credit_count);
    printf("STAT credit_total=%.2f\n", stats->credit_total);
}

/* ------------------------------------------------------------------ */
/* Program utama                                                       */
/* ------------------------------------------------------------------ */

int main(int argc, char *argv[])
{
    TransactionList list = {NULL, 0, 0};
    Statistics      stats;
    size_t          bad_line = 0;
    int             result;

    if (argc != 3) {
        fprintf(stderr, "Pemakaian: %s <transactions.csv> <processed_data.csv>\n",
                argc > 0 ? argv[0] : "processor");
        return EXIT_FILE_NOTFOUND;
    }

    result = list_init(&list, INITIAL_CAPACITY);
    if (result != EXIT_OK) {
        fprintf(stderr, "ERROR: gagal alokasi memori awal.\n");
        return EXIT_NO_MEMORY;
    }

    result = load_transactions(argv[1], &list, &bad_line);
    if (result != EXIT_OK) {
        switch (result) {
        case EXIT_FILE_NOTFOUND:
            fprintf(stderr, "ERROR: file input tidak ditemukan: %s\n", argv[1]);
            break;
        case EXIT_BAD_FORMAT:
            fprintf(stderr, "ERROR: format CSV tidak valid pada baris %lu.\n",
                    (unsigned long)bad_line);
            break;
        default:
            fprintf(stderr, "ERROR: gagal alokasi memori saat membaca baris %lu.\n",
                    (unsigned long)bad_line);
            break;
        }
        list_free(&list);
        return result;
    }

    calculate_all(&list);
    sort_by_date(&list);
    compute_statistics(&list, &stats);

    result = write_processed(argv[2], &list);
    if (result != EXIT_OK) {
        fprintf(stderr, "ERROR: gagal menulis file output: %s\n", argv[2]);
        list_free(&list);
        return result;
    }

    print_statistics(&stats);
    printf("Output: %s\n", argv[2]);

    list_free(&list); /* free() + set NULL */
    return EXIT_OK;
}
