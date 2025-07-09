export function padZeros(num: String, size: number): String {
  if (num !== undefined && num !== null && num !== "") {
    num = "" + num;
    while (num.length < size) num = "0" + num;
    return num;
  } else return num;
}