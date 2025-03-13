/**
 * Truncates text to a specified length and adds ellipsis if needed
 * @param text The text to truncate
 * @param maxLength The maximum length before truncation
 * @returns The truncated text with ellipsis if needed
 */
export default function truncateText(text: string, maxLength: number = 30): string {
  if (!text) return '';
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength) + '...';
} 