export default function formatDate(date: string | Date) {
  const months = [
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
  ];

  const day = new Date(date).getDate();
  const year = new Date(date).getFullYear();
  const month = months[new Date(date).getMonth()];
  const dateStr = `${month} ${day}, ${year}`;

  return dateStr;
}
