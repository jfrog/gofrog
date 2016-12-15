package fanout

import (
        "testing"
        "io"
        "bytes"
        "crypto/sha256"
        "encoding/hex"
)

const input = "yogreshobuddy!"
const checksum = "72a0230d6e5eebb437a9069ebb390171284192e9a993938d02cb0aaae003fd1c"

var (
        inputBytes = []byte(input)
)

func TestFanoutRead(t *testing.T) {
        proc := func(r io.Reader) (interface{}, error) {
                hash := sha256.New()
                if _, err := io.Copy(hash, r); err != nil {
                        t.Fatal(t)
                }
                return hash.Sum(nil), nil
        }

        //Using a closure argument instead of results
        var sum3 []byte
        proc1 := func(r io.Reader) (rt interface{}, er error) {
                hash := sha256.New()
                if _, err := io.Copy(hash, r); err != nil {
                        t.Fatal(t)
                }
                sum3 = hash.Sum(nil)
                return
        }

        r := bytes.NewReader(inputBytes)
        fr := NewFanoutReader(r, ReaderFunc(proc), ReaderFunc(proc), ReaderFunc(proc1))
        results, err := fr.ReadAll()

        if (err != nil) {
                t.Error(err)
        }
        sum1 := results[0].([]byte)
        sum2 := results[1].([]byte)

        sum1str := hex.EncodeToString(sum1)
        sum2str := hex.EncodeToString(sum2)
        sum3str := hex.EncodeToString(sum3)

        if !(sum1str == sum2str && sum1str == sum3str) {
                t.Errorf("Sum1 %s and sum2 %s and sum3 %s are not the same", sum1str, sum2str, sum3str)
        }

        if sum1str != checksum {
                t.Errorf("Checksum is not as expected: %s != %s", sum1str, checksum)
        }
}
