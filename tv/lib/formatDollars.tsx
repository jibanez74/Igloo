export default function formatDollars(amount: number | string): string {
  // Convert string to number if needed
  const numericAmount =
    typeof amount === "string" ? parseFloat(amount) : amount;

  // Handle invalid input
  if (isNaN(numericAmount)) {
    return "$0.00";
  }

  // Format using Intl.NumberFormat
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(numericAmount);
}
