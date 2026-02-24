type MovieDocumentMetaProps = {
  title: string;
  description: string;
};

export default function MovieDocumentMeta({
  title,
  description,
}: MovieDocumentMetaProps) {
  return (
    <>
      <title>{title}</title>
      <meta name="description" content={description} />
    </>
  );
}
