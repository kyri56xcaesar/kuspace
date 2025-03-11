#include <stdio.h>
#include <stdlib.h>
#include <ctype.h>

#define BUFFER_SIZE 1024 // Define a buffer size for reading chunks

// Function to convert a string to uppercase
void run(char *data) {
    for (int i = 0; data[i] != '\0'; i++) {
        data[i] = toupper((unsigned char)data[i]); // Convert to uppercase
    }
}

int main() {
    FILE *fp1, *fp2;
    fp1 = fopen("tests/input/job.in", "r");  // Open input file
    if (fp1 == NULL) {
        perror("Error opening input file");
        return 1;
    }

    fp2 = fopen("tests/output/job_c.out", "w");  // Open output file
    if (fp2 == NULL) {
        perror("Error opening output file");
        fclose(fp1);  // Close the input file before exiting
        return 1;
    }

    char buffer[BUFFER_SIZE];

    // Read file line by line and process it
    while (fgets(buffer, BUFFER_SIZE, fp1) != NULL) {
        run(buffer);         // Convert to uppercase
        fputs(buffer, fp2);  // Write to output file
    }

    // Close the files
    fclose(fp1);
    fclose(fp2);

    printf("Conversion completed! Check 'tests/output/job_c.out'\n");
    return 0;
}
