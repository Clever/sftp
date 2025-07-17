package sftp

import (
	"bytes"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestTranslatePath(t *testing.T) {
	testCases := []struct {
		prefix, desc, home, path, result string
	}{
		{
			desc:   "trivial path",
			home:   "sftp/test_user",
			path:   "file",
			result: "sftp/test_user/file",
		},
		{
			desc:   "trivial directory",
			home:   "sftp/test_user",
			path:   "dir/",
			result: "sftp/test_user/dir/",
		},
		{
			desc:   "nested path",
			home:   "sftp/test_user",
			path:   "dir/file",
			result: "sftp/test_user/dir/file",
		},
		{
			desc:   "path w/ ..",
			home:   "sftp/test_user",
			path:   "dir/../some_other_file",
			result: "sftp/test_user/some_other_file",
		},
		{
			desc:   "handle escaping attempt",
			prefix: "sftp",
			home:   "/test_user",
			path:   "dir/../../some_escape_attempt",
			result: "sftp/test_user/some_escape_attempt",
		},
		{
			desc:   "convoluted escape attempt",
			prefix: "sftp",
			home:   "/test_user",
			path:   "///dir/./../../../another_escape_attempt",
			result: "sftp", // ends up w/ the base path
		},
	}

	for _, spec := range testCases {
		t.Run(spec.desc, func(t *testing.T) {
			path, err := TranslatePath(spec.prefix, spec.home, spec.path)
			assert.NoError(t, err)
			assert.Equal(t, spec.result, path)
		})
	}
}

func TestStat(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
		Bucket:  aws.String("bucket"),
		Prefix:  aws.String("home/dir/file"),
		MaxKeys: aws.Int32(1),
	}).Return(&s3.ListObjectsV2Output{
		KeyCount: aws.Int32(1),
		Contents: []types.Object{
			{Key: aws.String("file"), Size: aws.Int64(123), LastModified: aws.Time(time.Now())},
		},
	}, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	info, err := driver.Stat("../../dir/file")

	assert.NoError(t, err)
	assert.Equal(t, info.Name(), "home/dir/file")
	assert.Equal(t, info.IsDir(), false)
}

func TestListDir(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().ListObjectsV2(gomock.Any(), &s3.ListObjectsV2Input{
		Bucket:    aws.String("bucket"),
		Prefix:    aws.String("home/dir/"),
		Delimiter: aws.String("/"),
	}).Return(&s3.ListObjectsV2Output{
		KeyCount: aws.Int32(1),
		Contents: []types.Object{
			{Key: aws.String("file"), Size: aws.Int64(123), LastModified: aws.Time(time.Now())},
			{Key: aws.String("other_file"), Size: aws.Int64(456), LastModified: aws.Time(time.Now())},
		},
		CommonPrefixes: []types.CommonPrefix{
			{Prefix: aws.String("nested_dir/")},
		},
		IsTruncated: aws.Bool(false),
	}, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	files, err := driver.ListDir("../../dir/")

	assert.NoError(t, err)
	assert.Equal(t, len(files), 3)
	assert.Equal(t, files[0].Name(), "file")
	assert.Equal(t, files[0].IsDir(), false)
	assert.Equal(t, files[1].Name(), "other_file")
	assert.Equal(t, files[1].IsDir(), false)
	assert.Equal(t, files[2].Name(), "nested_dir")
	assert.Equal(t, files[2].IsDir(), true)
}

func TestDeleteDir(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().DeleteObject(gomock.Any(), &s3.DeleteObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("home/dir/"),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.DeleteFile("../../dir/")

	assert.NoError(t, err)
}

func TestDeleteDirImplicitSlash(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().DeleteObject(gomock.Any(), &s3.DeleteObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("home/dir/"),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.DeleteDir("../../dir")

	assert.NoError(t, err)
}

func TestDeleteFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().DeleteObject(gomock.Any(), &s3.DeleteObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("home/dir/file"),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.DeleteFile("../../dir/file")

	assert.NoError(t, err)
}

func TestRename(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().CopyObject(gomock.Any(), &s3.CopyObjectInput{
		Bucket:               aws.String("bucket"),
		CopySource:           aws.String("bucket/home/dir/file"),
		Key:                  aws.String("home/dir/new_file"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	}).Return(nil, nil)

	mockS3API.EXPECT().DeleteObject(gomock.Any(), &s3.DeleteObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("home/dir/file"),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.Rename("dir/file", "dir/new_file")

	assert.NoError(t, err)
}

func TestRelativeMakeDir(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().PutObject(gomock.Any(), &s3.PutObjectInput{
		Bucket:               aws.String("bucket"),
		Key:                  aws.String("home/new_dir/"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Body:                 bytes.NewReader([]byte{}),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	assert.NoError(t, driver.MakeDir("new_dir"))
}

func TestAbsoluteMakeDir(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().PutObject(gomock.Any(), &s3.PutObjectInput{
		Bucket:               aws.String("bucket"),
		Key:                  aws.String("new_dir/"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Body:                 bytes.NewReader([]byte{}),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.MakeDir("/new_dir")

	assert.NoError(t, err)
}

func TestGetFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().GetObject(gomock.Any(), &s3.GetObjectInput{
		Bucket: aws.String("bucket"),
		Key:    aws.String("home/dir/file"),
	}).Return(&s3.GetObjectOutput{
		Body: nil,
	}, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	_, err := driver.GetFile("../../dir/file")

	assert.NoError(t, err)
}

type testLogger struct{}

func (l *testLogger) InfoD(title string, meta map[string]interface{}) {}
func (l *testLogger) ErrorD(title string, meta map[string]interface{}) {}

func TestGetFileFromBlockedIPAddress(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	BLOCK_DOWNLOADS_IP_ADDRESSES = []string{"1.1.1.1"}

	driver := &S3Driver{
		s3:              mockS3API,
		bucket:          "bucket",
		homePath:        "home",
		remoteIPAddress: "1.1.1.1:1234",
		lg:              &testLogger{},
	}
	_, err := driver.GetFile("../../dir/file")

	assert.Error(t, err)

	BLOCK_DOWNLOADS_IP_ADDRESSES = []string{}
}

func TestPutFile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)

	mockS3API.EXPECT().PutObject(gomock.Any(), &s3.PutObjectInput{
		Bucket:               aws.String("bucket"),
		Key:                  aws.String("home/dir/file"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
		Body:                 bytes.NewReader([]byte{1, 2, 3}),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
	}
	err := driver.PutFile("../../dir/file", bytes.NewReader([]byte{1, 2, 3}))

	assert.NoError(t, err)
}

func TestPutFileWithKmsKeyID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockS3API := NewMockS3API(mockCtrl)
	kmsKeyID := "123456"

	mockS3API.EXPECT().PutObject(gomock.Any(), &s3.PutObjectInput{
		Bucket:               aws.String("bucket"),
		Key:                  aws.String("home/dir/file"),
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:          aws.String(kmsKeyID),
		Body:                 bytes.NewReader([]byte{1, 2, 3}),
	}).Return(nil, nil)

	driver := &S3Driver{
		s3:       mockS3API,
		bucket:   "bucket",
		homePath: "home",
		kmsKeyID: &kmsKeyID,
	}
	err := driver.PutFile("../../dir/file", bytes.NewReader([]byte{1, 2, 3}))

	assert.NoError(t, err)
}
