CREATE TABLE "demo" (
    "id" uuid PRIMARY KEY,
    "username" varchar(50) NOT NULL,
    "amount" decimal(32, 16) CHECK (amount > 0) NOT NULL
);