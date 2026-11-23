/*
 * processor.h - POS Transaction Processing Engine (Bahasa C)
 *
 * Definisi struct, konstanta exit code, dan prototipe fungsi
 * manajemen memori dinamis (malloc / calloc / realloc / free).
 */
#ifndef PROCESSOR_H
#define PROCESSOR_H

#include <stddef.h>
#include <stdio.h>

/* ---- Exit code (dibaca oleh orkestrator Go) ---- */
#define EXIT_OK            0 /* Sukses                          */
#define EXIT_FILE_NOTFOUND 1 /* File input tidak ditemukan      */
#define EXIT_BAD_FORMAT    2 /* Format CSV tidak valid          */
#define EXIT_NO_MEMORY     3 /* Gagal alokasi memori            */
#define EXIT_WRITE_FAILED  4 /* Gagal menulis file output       */

/* ---- Ukuran buffer field (termasuk terminator '\0') ---- */
#define ID_LEN        16
#define NAME_LEN      101
#define CATEGORY_LEN  51
#define PAYMENT_LEN   21
#define DATE_LEN      16
#define CASHIER_LEN   101

#define INPUT_FIELD_COUNT 10
#define INITIAL_CAPACITY  16
#define MAX_GROUPS        3

#define INPUT_HEADER \
    "transaction_id,product_id,product_name,category,quantity,unit_price," \
    "discount_percent,payment_method,transaction_date,cashier_name"
#define OUTPUT_HEADER \
    "transaction_id,product_name,category,quantity,subtotal,discount_amount," \
    "net_total,payment_method,transaction_date"

/* Satu record transaksi (input + hasil kalkulasi). */
typedef struct {
    char   transaction_id[ID_LEN];
    char   product_id[ID_LEN];
    char   product_name[NAME_LEN];
    char   category[CATEGORY_LEN];
    int    quantity;
    double unit_price;
    double discount_percent;
    char   payment_method[PAYMENT_LEN];
    char   transaction_date[DATE_LEN];
    char   cashier_name[CASHIER_LEN];
    /* hasil kalkulasi */
    double subtotal;
    double discount_amount;
    double net_total;
    size_t sequence; /* urutan baca, menjaga sorting tetap stabil */
} Transaction;

/* Array dinamis transaksi. */
typedef struct {
    Transaction *items;
    size_t       count;
    size_t       capacity;
} TransactionList;

/* Statistik agregat. */
typedef struct {
    size_t total_records;
    long   total_quantity;
    double total_subtotal;
    double total_discount;
    double total_net;
    double average_net;
    double max_net;
    double min_net;
    char   max_id[ID_LEN];
    char   min_id[ID_LEN];
    size_t cash_count;
    size_t debit_count;
    size_t credit_count;
    double cash_total;
    double debit_total;
    double credit_total;
} Statistics;

/* ---- Manajemen memori dinamis ---- */
int  list_init(TransactionList *list, size_t initial_capacity); /* calloc  */
int  list_push(TransactionList *list, const Transaction *trx);  /* realloc */
void list_free(TransactionList *list);                          /* free + NULL */

/* ---- Pembacaan & parsing CSV ---- */
char *read_line(FILE *fp, int *status);                          /* malloc/realloc */
int   split_csv_line(const char *line, char ***fields_out, size_t *count_out);
void  free_fields(char ***fields, size_t count);
int   parse_transaction(char **fields, size_t count, Transaction *out);
int   load_transactions(const char *path, TransactionList *list, size_t *bad_line);

/* ---- Kalkulasi, sorting, statistik ---- */
void calculate_transaction(Transaction *trx);
void calculate_all(TransactionList *list);
void sort_by_date(TransactionList *list);
void compute_statistics(const TransactionList *list, Statistics *stats);

/* ---- Output ---- */
int  write_processed(const char *path, const TransactionList *list);
void print_statistics(const Statistics *stats);

#endif /* PROCESSOR_H */
