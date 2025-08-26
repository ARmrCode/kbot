pipeline {
    agent {
        kubernetes {
            label 'golang-agent'
            defaultContainer 'golang'
            yaml """
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: golang
    image: golang:1.23
    command:
    - cat
    tty: true
    volumeMounts:
    - mountPath: /home/jenkins/agent
      name: workspace-volume
  - name: docker
    image: docker:27.2.0
    command:
    - cat
    tty: true
    volumeMounts:
    - mountPath: /var/run/docker.sock
      name: docker-sock
  volumes:
  - name: workspace-volume
    emptyDir: {}
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
"""
        }
    }

    parameters {
        choice(name: 'OS', choices: ['linux'], description: 'Target OS')
        choice(name: 'ARCH', choices: ['amd64', 'arm64'], description: 'Target architecture')
    }

    environment {
        DOCKER_CLI_EXPERIMENTAL = "enabled"
        DOCKER_BUILDKIT = "1"
        REGISTRY = "ghcr.io"
        REPO = "armrcode/kbot"
        IMAGE = "${env.REGISTRY}/${env.REPO}"
    }

    stages {
        stage('Checkout SCM') {
            steps {
                checkout scm
                sh 'git config --global --add safe.directory ${WORKSPACE}'
            }
        }

        stage('Lint') {
            steps {
                container('golang') {
                    sh '''
                        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.60.3
                        ./bin/golangci-lint run --timeout=5m -v
                    '''
                }
            }
        }

        stage('Test') {
            steps {
                container('golang') {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build Binary') {
            steps {
                container('golang') {
                    sh "make build TARGETOS=${params.OS} TARGETARCH=${params.ARCH}"
                }
            }
        }

        stage('Docker Build & Push') {
            steps {
                container('docker') {
                    withCredentials([string(credentialsId: 'ghcr_pat', variable: 'CR_PAT')]) {
                        sh '''
                            VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0-$(git rev-parse --short HEAD))
                            echo "Building image $IMAGE:$VERSION for ${OS}/${ARCH}"
                            echo $CR_PAT | docker login ghcr.io -u ${GITHUB_ACTOR:-jenkins} --password-stdin
                            docker buildx create --use --name multiarch || true
                            docker buildx build \
                                --platform linux/${ARCH} \
                                -t $IMAGE:$VERSION \
                                -t $IMAGE:latest \
                                --push .
                            echo $VERSION > .image_version
                        '''
                    }
                }
            }
        }

        stage('Update Helm Chart') {
            steps {
                container('golang') {
                    withCredentials([string(credentialsId: 'ghcr_pat', variable: 'CR_PAT')]) {
                        sh '''
                            VERSION=$(cat .image_version)
                            echo "Updating helm/values.yaml with tag ${VERSION} and arch ${ARCH}"
                            yq -i ".image.tag = \\"${VERSION}\\" | .image.arch = \\"${ARCH}\\"" helm/values.yaml
                            git config user.name "jenkins"
                            git config user.email "jenkins@local"
                            git add helm/values.yaml
                            git commit -m "Update Helm image tag to ${VERSION}" || echo "No changes to commit"
                            git push https://$CR_PAT@github.com/ARmrCode/kbot.git HEAD:main
                        '''
                    }
                }
            }
        }
    }
}
