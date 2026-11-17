export function readFileBase64(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("invalid"));
    reader.onload = () => {
      const content =
        typeof reader.result === "string" ? reader.result.split(",")[1] : "";
      if (content) resolve(content);
      else reject(new Error("invalid"));
    };
    reader.readAsDataURL(file);
  });
}
