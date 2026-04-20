package xmlresp

import (
	"fmt"
	"time"

	"s3server/storage"
)

func ListBuckets(buckets []string, dataDir string) string {
	result := `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>owner</ID>
    <DisplayName>owner</DisplayName>
  </Owner>
  <Buckets>`

	for _, bucket := range buckets {
		result += fmt.Sprintf(`
    <Bucket>
      <Name>%s</Name>
      <CreationDate>%s</CreationDate>
    </Bucket>`, bucket, time.Now().Format(time.RFC3339))
	}

	result += `
  </Buckets>
</ListAllMyBucketsResult>`

	return result
}

func ListObjectsV2(objects map[string][]storage.ObjectMeta, bucket, prefix string, maxKeys int, isTruncated bool, startAfter string) string {
	result := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ListObjectsV2Result xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>%s</Name>
  <Prefix>%s</Prefix>
  <MaxKeys>%d</MaxKeys>
  <IsTruncated>%v</IsTruncated>`,
		bucket, prefix, maxKeys, isTruncated)

	if len(objects) > 0 {
		result += "\n  <Contents>"
		for key, metas := range objects {
			meta := metas[0]
			result += fmt.Sprintf(`
    <Key>%s</Key>
    <LastModified>%s</LastModified>
    <ETag>%s</ETag>
    <Size>%d</Size>
    <StorageClass>STANDARD</StorageClass>`,
				key, meta.LastModified.Format(time.RFC3339), meta.ETag, meta.Size)
		}
		result += "\n  </Contents>"
	}

	result += "\n</ListObjectsV2Result>"
	return result
}
