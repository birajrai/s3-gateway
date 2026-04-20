use crate::storage::ObjectMeta;

pub fn list_buckets(buckets: &[String]) -> String {
    let mut xml = String::from(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>owner</ID>
    <DisplayName>owner</DisplayName>
  </Owner>
  <Buckets>"#,
    );

    let now = chrono::Utc::now().to_rfc3339();

    for bucket in buckets {
        xml.push_str(&format!(
            "
    <Bucket>
      <Name>{}</Name>
      <CreationDate>{}</CreationDate>
    </Bucket>",
            bucket, now
        ));
    }

    xml.push_str(
        "
  </Buckets>
</ListAllMyBucketsResult>",
    );

    xml
}

pub fn list_objects_v2(
    objects: &[ObjectMeta],
    bucket: &str,
    prefix: &str,
    max_keys: usize,
    is_truncated: bool,
) -> String {
    let mut xml = format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<ListObjectsV2Result xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>{}</Name>
  <Prefix>{}</Prefix>
  <MaxKeys>{}</MaxKeys>
  <IsTruncated>{}</IsTruncated>"#,
        bucket, prefix, max_keys, is_truncated
    );

    if !objects.is_empty() {
        xml.push_str("\n  <Contents>");
        for obj in objects {
            let last_modified = obj
                .last_modified
                .map(|t| {
                    let datetime: chrono::DateTime<chrono::Utc> = t.into();
                    datetime.to_rfc3339()
                })
                .unwrap_or_default();

            xml.push_str(&format!(
                "
    <Key>{}</Key>
    <LastModified>{}</LastModified>
    <ETag>\"{}\"</ETag>
    <Size>{}</Size>
    <StorageClass>STANDARD</StorageClass>",
                obj.key, last_modified, obj.etag, obj.size
            ));
        }
        xml.push_str("\n  </Contents>");
    }

    xml.push_str("\n</ListObjectsV2Result>");

    xml
}

pub fn create_multipart_upload(bucket: &str, key: &str, upload_id: &str) -> String {
    format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<CreateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Bucket>{}</Bucket>
  <Key>{}</Key>
  <UploadId>{}</UploadId>
</CreateMultipartUploadResult>"#,
        bucket, key, upload_id
    )
}

pub fn complete_multipart_upload(bucket: &str, key: &str, etag: &str, size: u64) -> String {
    format!(
        r#"<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Location>http://localhost/{}/{}</Location>
  <Bucket>{}</Bucket>
  <Key>{}</Key>
  <ETag>\"{}\"</ETag>
</CompleteMultipartUploadResult>"#,
        bucket, key, bucket, key, etag
    )
}
